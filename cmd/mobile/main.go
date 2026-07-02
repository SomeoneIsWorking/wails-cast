package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"wails-cast/pkg/castapi"
	"wails-cast/pkg/options"
)

const (
	prefBaseURL = "baseURL"
	prefToken   = "token"
)

type ui struct {
	fyneApp fyne.App
	window  fyne.Window
	client  *castapi.Client

	tabs *container.AppTabs

	devicesMu sync.Mutex
	devices   []castapi.RemoteDevice
	deviceSel *widget.Select

	// Library tab
	libraryItems []castapi.LibraryItem
	libraryList  *widget.List

	// Torrents tab
	torrents    []castapi.TorrentStatus
	torrentList *widget.List
	torrentStop chan struct{}

	// Library Mgmt tab
	currentTree      *castapi.LibraryScanResult
	mgmtTree         *widget.Tree
	mgmtIdentifyBtn  *widget.Button
	mgmtPreviewBtn   *widget.Button
	seasonStop       chan struct{}
	seasonBarLabel   *widget.Label
	seasonBarWrapper *fyne.Container

	// Subtitle state
	currentSubtitlePath string
	currentSubtitleOpts options.SubtitleCastOptions

	// Now Playing poller
	nowPlayingStop chan struct{}
}

func main() {
	a := app.NewWithID("io.wailscast.remote")
	w := a.NewWindow("wails-cast remote")
	w.Resize(fyne.NewSize(420, 720))

	u := &ui{fyneApp: a, window: w}
	u.showConnect()

	w.ShowAndRun()
}

func (u *ui) fail(err error) { dialog.ShowError(err, u.window) }

// ---------------------------------------------------------------------------
// Connect screen
// ---------------------------------------------------------------------------

func (u *ui) showConnect() {
	prefs := u.fyneApp.Preferences()

	host := widget.NewEntry()
	host.SetPlaceHolder("host")
	port := widget.NewEntry()
	port.SetPlaceHolder("port")
	token := widget.NewPasswordEntry()
	token.SetPlaceHolder("token (optional)")

	if last := prefs.String(prefBaseURL); last != "" {
		host.SetText(last)
	}
	if lastTok := prefs.String(prefToken); lastTok != "" {
		token.SetText(lastTok)
	}

	var instances []castapi.CastInstance
	list := widget.NewList(
		func() int { return len(instances) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			inst := instances[i]
			o.(*widget.Label).SetText(fmt.Sprintf("%s  (%s:%d)", inst.Name, inst.Host, inst.Port))
		},
	)
	list.OnSelected = func(i widget.ListItemID) {
		list.Unselect(i)
		if i < 0 || i >= len(instances) {
			return
		}
		inst := instances[i]
		prefs.SetString(prefBaseURL, inst.URL)
		prefs.SetString(prefToken, token.Text)
		u.client = castapi.New(inst.URL, token.Text)
		u.showMain()
	}

	discover := widget.NewButton("Discover", func() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			found, err := castapi.Discover(ctx)
			if err != nil {
				fyne.Do(func() { u.fail(err) })
				return
			}
			fyne.Do(func() { instances = found; list.Refresh() })
		}()
	})

	connect := widget.NewButton("Connect", func() {
		p, err := strconv.Atoi(port.Text)
		if err != nil && port.Text != "" {
			u.fail(fmt.Errorf("invalid port: %v", err))
			return
		}
		base := host.Text
		if base == "" {
			u.fail(fmt.Errorf("host is required"))
			return
		}
		if p > 0 {
			base = fmt.Sprintf("http://%s:%d", host.Text, p)
		}
		prefs.SetString(prefBaseURL, base)
		prefs.SetString(prefToken, token.Text)
		u.client = castapi.New(base, token.Text)
		u.showMain()
	})

	form := container.NewVBox(
		widget.NewLabel("Manual connect"),
		host, port, token, connect,
	)
	top := container.NewVBox(
		widget.NewLabel("Discover on LAN"),
		discover,
	)
	u.window.SetContent(container.NewBorder(top, form, nil, nil, list))
}

// ---------------------------------------------------------------------------
// Main tabs
// ---------------------------------------------------------------------------

func (u *ui) showMain() {
	libTab := container.NewTabItem("Library", u.buildLibraryTab())
	torTab := container.NewTabItem("Torrents", u.buildTorrentsTab())
	mgmtTab := container.NewTabItem("Library Mgmt", u.buildMgmtTab())
	u.tabs = container.NewAppTabs(libTab, torTab, mgmtTab)
	// Poll torrents only when the Torrents tab is visible.
	u.tabs.OnSelected = func(t *container.TabItem) {
		if t == torTab {
			u.startTorrentPoll()
		} else {
			u.stopTorrentPoll()
		}
	}
	u.window.SetContent(u.tabs)
	u.window.SetOnClosed(func() {
		u.stopTorrentPoll()
		u.stopSeasonPoll()
		u.stopNowPlayingPoll()
	})
}

func (u *ui) stopNowPlayingPoll() {
	if u.nowPlayingStop == nil {
		return
	}
	close(u.nowPlayingStop)
	u.nowPlayingStop = nil
}

// ---------------------------------------------------------------------------
// Library tab
// ---------------------------------------------------------------------------

func (u *ui) buildLibraryTab() fyne.CanvasObject {
	u.deviceSel = widget.NewSelect([]string{"Local"}, func(string) {})
	u.deviceSel.SetSelectedIndex(0)

	u.libraryList = widget.NewList(
		func() int { return len(u.libraryItems) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(u.libraryItems[i].Name)
		},
	)
	u.libraryList.OnSelected = func(i widget.ListItemID) {
		u.libraryList.Unselect(i)
		if i < 0 || i >= len(u.libraryItems) {
			return
		}
		u.showPlayDialog(u.libraryItems[i])
	}

	refresh := widget.NewButton("Refresh", func() {
		go u.reloadLibrary()
		go u.refreshDevices()
	})
	disconnect := widget.NewButton("Disconnect", func() { u.showConnect() })

	top := container.NewVBox(
		container.NewHBox(disconnect, refresh),
		widget.NewLabel("Output device:"),
		u.deviceSel,
	)

	go u.reloadLibrary()
	go u.refreshDevices()

	return container.NewBorder(top, nil, nil, nil, u.libraryList)
}

func (u *ui) reloadLibrary() {
	items, err := u.client.Library()
	if err != nil {
		fyne.Do(func() { u.fail(err) })
		return
	}
	fyne.Do(func() { u.libraryItems = items; u.libraryList.Refresh() })
}

func (u *ui) refreshDevices() {
	devs, err := u.client.Devices()
	if err != nil {
		fyne.Do(func() { u.fail(err) })
		return
	}
	u.devicesMu.Lock()
	u.devices = devs
	u.devicesMu.Unlock()

	opts := []string{"Local"}
	for _, d := range devs {
		if d.Host == "local" {
			continue
		}
		opts = append(opts, d.Name)
	}
	fyne.Do(func() {
		u.deviceSel.Options = opts
		u.deviceSel.SetSelectedIndex(0)
		u.deviceSel.Refresh()
	})
}

func (u *ui) selectedDeviceHost() string {
	u.devicesMu.Lock()
	defer u.devicesMu.Unlock()
	idx := u.deviceSel.SelectedIndex()
	if idx <= 0 {
		return "local"
	}
	pos := 0
	for _, d := range u.devices {
		if d.Host == "local" {
			continue
		}
		pos++
		if pos == idx {
			return d.Host
		}
	}
	return "local"
}

func (u *ui) showPlayDialog(item castapi.LibraryItem) {
	go func() {
		info, err := u.client.TrackInfo(item.ID)
		if err != nil {
			fyne.Do(func() { u.fail(err) })
			return
		}

		fyne.Do(func() {
			audioOptions := []string{"Default"}
			for _, a := range info.AudioTracks {
				audioOptions = append(audioOptions, fmt.Sprintf("%d: %s", a.Index, a.Language))
			}
			audioSel := widget.NewSelect(audioOptions, func(string) {})
			audioSel.SetSelectedIndex(0)

			subOptions := []string{"None"}
			for _, s := range info.SubtitleTracks {
				subOptions = append(subOptions, s.Label)
			}
			subSel := widget.NewSelect(subOptions, func(string) {})
			subSel.SetSelectedIndex(0)

			qualityOptions := []string{"Default", "8M", "5M", "3M", "2M"}
			qualitySel := widget.NewSelect(qualityOptions, func(string) {})
			qualitySel.SetSelectedIndex(0)

			form := container.NewVBox(
				widget.NewLabel("Audio track"), audioSel,
				widget.NewLabel("Subtitle"), subSel,
				widget.NewLabel("Quality"), qualitySel,
			)

			dialog.ShowCustomConfirm("Play "+item.Name, "Play", "Cancel", form, func(ok bool) {
				if !ok {
					return
				}
				opts := castapi.PlayOptions{}
				if idx := audioSel.SelectedIndex(); idx > 0 && idx-1 < len(info.AudioTracks) {
					opts.AudioTrack = info.AudioTracks[idx-1].Index
				}
				if idx := subSel.SelectedIndex(); idx > 0 && idx-1 < len(info.SubtitleTracks) {
					opts.SubtitlePath = info.SubtitleTracks[idx-1].Path
				}
				if idx := qualitySel.SelectedIndex(); idx > 0 {
					q := qualityOptions[idx]
					opts.Quality = &q
				}
				u.currentSubtitlePath = opts.SubtitlePath
				u.currentSubtitleOpts = options.SubtitleCastOptions{Path: opts.SubtitlePath, FontSize: 24}
				go func() {
					if _, err := u.client.Play(item.ID, u.selectedDeviceHost(), opts); err != nil {
						fyne.Do(func() { u.fail(err) })
						return
					}
					fyne.Do(func() { u.showNowPlaying() })
				}()
			}, u.window)
		})
	}()
}

// ---------------------------------------------------------------------------
// Torrents tab
// ---------------------------------------------------------------------------

func (u *ui) buildTorrentsTab() fyne.CanvasObject {
	u.torrentList = widget.NewList(
		func() int { return len(u.torrents) },
		func() fyne.CanvasObject {
			return container.NewVBox(widget.NewLabel(""), widget.NewProgressBar(), widget.NewLabel(""))
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			box := o.(*fyne.Container)
			t := u.torrents[i]
			box.Objects[0].(*widget.Label).SetText(truncate(t.Name, 40))
			box.Objects[1].(*widget.ProgressBar).SetValue(t.Progress)
			box.Objects[2].(*widget.Label).SetText(fmt.Sprintf("%s · %s · %s", t.State, fmtSpeed(t.DlSpeed), fmtEta(t.Eta)))
		},
	)

	magnet := widget.NewEntry()
	magnet.SetPlaceHolder("magnet link")
	add := widget.NewButton("Add", func() {
		m := strings.TrimSpace(magnet.Text)
		if m == "" {
			return
		}
		go func() {
			if err := u.client.AddTorrent(m); err != nil {
				fyne.Do(func() { u.fail(err) })
				return
			}
			fyne.Do(func() { magnet.SetText("") })
			u.pollTorrentsOnce()
		}()
	})
	bottom := container.NewBorder(nil, nil, nil, add, magnet)
	return container.NewBorder(nil, bottom, nil, nil, u.torrentList)
}

func (u *ui) startTorrentPoll() {
	if u.torrentStop != nil {
		return
	}
	u.torrentStop = make(chan struct{})
	stop := u.torrentStop
	u.pollTorrentsOnce()
	go func() {
		t := time.NewTicker(3 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				u.pollTorrentsOnce()
			}
		}
	}()
}

func (u *ui) stopTorrentPoll() {
	if u.torrentStop == nil {
		return
	}
	close(u.torrentStop)
	u.torrentStop = nil
}

func (u *ui) pollTorrentsOnce() {
	go func() {
		list, err := u.client.Torrents()
		if err != nil {
			return
		}
		fyne.Do(func() { u.torrents = list; u.torrentList.Refresh() })
	}()
}

// ---------------------------------------------------------------------------
// Library Mgmt tab
// ---------------------------------------------------------------------------

func (u *ui) buildMgmtTab() fyne.CanvasObject {
	scan := widget.NewButton("Scan tree", func() {
		go func() {
			tree, err := u.client.LibraryTree()
			if err != nil {
				fyne.Do(func() { u.fail(err) })
				return
			}
			fyne.Do(func() {
				u.currentTree = tree
				u.mgmtIdentifyBtn.Enable()
				u.mgmtPreviewBtn.Enable()
				u.mgmtTree.Refresh()
			})
		}()
	})
	u.mgmtIdentifyBtn = widget.NewButton("Identify (TMDB)", func() {
		if u.currentTree == nil {
			return
		}
		go func() {
			out, err := u.client.Identify(*u.currentTree)
			if err != nil {
				fyne.Do(func() { u.fail(err) })
				return
			}
			fyne.Do(func() { u.currentTree = out; u.mgmtTree.Refresh() })
		}()
	})
	u.mgmtPreviewBtn = widget.NewButton("Preview organize", func() {
		if u.currentTree == nil {
			return
		}
		go func() {
			plan, err := u.client.OrganizePreview(*u.currentTree)
			if err != nil {
				fyne.Do(func() { u.fail(err) })
				return
			}
			fyne.Do(func() { u.showOrganizeDialog(plan) })
		}()
	})
	u.mgmtIdentifyBtn.Disable()
	u.mgmtPreviewBtn.Disable()

	buttons := container.NewGridWithColumns(3, scan, u.mgmtIdentifyBtn, u.mgmtPreviewBtn)

	u.mgmtTree = widget.NewTree(
		u.treeChildUIDs,
		u.treeIsBranch,
		u.treeCreate,
		u.treeUpdate,
	)

	u.seasonBarLabel = widget.NewLabel("")
	cancel := widget.NewButton("Cancel", func() {
		go func() {
			if err := u.client.SeasonCancel(); err != nil {
				fyne.Do(func() { u.fail(err) })
			}
		}()
	})
	u.seasonBarWrapper = container.NewBorder(nil, nil, nil, cancel, u.seasonBarLabel)
	u.seasonBarWrapper.Hide()

	bottom := container.NewVBox(u.seasonBarWrapper)
	return container.NewBorder(buttons, bottom, nil, nil, u.mgmtTree)
}

func (u *ui) treeChildUIDs(uid widget.TreeNodeID) []widget.TreeNodeID {
	if u.currentTree == nil {
		return nil
	}
	if uid == "" {
		ids := make([]string, 0, len(u.currentTree.Shows))
		for i := range u.currentTree.Shows {
			ids = append(ids, fmt.Sprintf("s:%d", i))
		}
		return ids
	}
	parts := strings.Split(uid, ":")
	switch parts[0] {
	case "s":
		si, _ := strconv.Atoi(parts[1])
		if si < 0 || si >= len(u.currentTree.Shows) {
			return nil
		}
		ids := make([]string, 0, len(u.currentTree.Shows[si].Seasons))
		for j := range u.currentTree.Shows[si].Seasons {
			ids = append(ids, fmt.Sprintf("e:%d:%d", si, j))
		}
		return ids
	case "e":
		si, _ := strconv.Atoi(parts[1])
		sj, _ := strconv.Atoi(parts[2])
		if si < 0 || si >= len(u.currentTree.Shows) || sj < 0 || sj >= len(u.currentTree.Shows[si].Seasons) {
			return nil
		}
		ids := make([]string, 0)
		for k := range u.currentTree.Shows[si].Seasons[sj].Episodes {
			ids = append(ids, fmt.Sprintf("p:%d:%d:%d", si, sj, k))
		}
		return ids
	}
	return nil
}

func (u *ui) treeIsBranch(uid widget.TreeNodeID) bool {
	if uid == "" {
		return true
	}
	return strings.HasPrefix(uid, "s:") || strings.HasPrefix(uid, "e:")
}

func (u *ui) treeCreate(branch bool) fyne.CanvasObject {
	if branch {
		return container.NewHBox(widget.NewLabel(""), widget.NewButton("Translate…", nil))
	}
	return widget.NewLabel("")
}

func (u *ui) treeUpdate(uid widget.TreeNodeID, branch bool, o fyne.CanvasObject) {
	if u.currentTree == nil {
		return
	}
	if !branch {
		parts := strings.Split(uid, ":")
		si, _ := strconv.Atoi(parts[1])
		sj, _ := strconv.Atoi(parts[2])
		k, _ := strconv.Atoi(parts[3])
		if si >= len(u.currentTree.Shows) || sj >= len(u.currentTree.Shows[si].Seasons) || k >= len(u.currentTree.Shows[si].Seasons[sj].Episodes) {
			return
		}
		ep := u.currentTree.Shows[si].Seasons[sj].Episodes[k]
		name := ep.Name
		if ep.EpisodeName != "" {
			name = fmt.Sprintf("S%02dE%02d %s", ep.Season, ep.Episode, ep.EpisodeName)
		}
		o.(*widget.Label).SetText(name)
		return
	}
	box := o.(*fyne.Container)
	lbl := box.Objects[0].(*widget.Label)
	btn := box.Objects[1].(*widget.Button)
	parts := strings.Split(uid, ":")
	if parts[0] == "s" {
		si, _ := strconv.Atoi(parts[1])
		if si >= len(u.currentTree.Shows) {
			return
		}
		lbl.SetText(u.currentTree.Shows[si].Name)
		btn.Hide()
		return
	}
	si, _ := strconv.Atoi(parts[1])
	sj, _ := strconv.Atoi(parts[2])
	if si >= len(u.currentTree.Shows) || sj >= len(u.currentTree.Shows[si].Seasons) {
		return
	}
	show := u.currentTree.Shows[si]
	season := show.Seasons[sj]
	lbl.SetText(season.Name)
	btn.OnTapped = func() { u.promptTranslateSeason(show.Name, season) }
	btn.Show()
}

func (u *ui) showOrganizeDialog(plan []castapi.OrganizeMove) {
	list := widget.NewList(
		func() int { return len(plan) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(plan[i].Description)
		},
	)
	scroll := container.NewScroll(list)
	scroll.SetMinSize(fyne.NewSize(360, 400))
	dialog.ShowCustomConfirm("Organize plan", "Execute", "Cancel", scroll, func(ok bool) {
		if !ok {
			return
		}
		go func() {
			if err := u.client.OrganizeExecute(plan); err != nil {
				fyne.Do(func() { u.fail(err) })
			}
		}()
	}, u.window)
}

func (u *ui) promptTranslateSeason(showName string, season castapi.LibrarySeason) {
	langEntry := widget.NewEntry()
	langEntry.SetText("Turkish")
	dialog.ShowCustomConfirm("Translate "+season.Name, "Start", "Cancel",
		container.NewVBox(widget.NewLabel("Language"), langEntry),
		func(ok bool) {
			if !ok {
				return
			}
			lang := strings.TrimSpace(langEntry.Text)
			if lang == "" {
				lang = "Turkish"
			}
			paths := make([]string, 0, len(season.Episodes))
			for _, e := range season.Episodes {
				paths = append(paths, e.Path)
			}
			go func() {
				if err := u.client.TranslateSeason(showName, season.Name, paths, lang); err != nil {
					fyne.Do(func() { u.fail(err) })
					return
				}
				fyne.Do(func() { u.startSeasonPoll() })
			}()
		}, u.window)
}

func (u *ui) startSeasonPoll() {
	u.stopSeasonPoll()
	u.seasonStop = make(chan struct{})
	stop := u.seasonStop
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				st, err := u.client.SeasonStatus()
				if err != nil {
					continue
				}
				running := st.Status == "running"
				text := fmt.Sprintf("Translating %s · %d/%d · %s", st.SeasonName, st.CurrentEpisode, st.TotalEpisodes, st.Message)
				fyne.Do(func() {
					if running {
						u.seasonBarLabel.SetText(text)
						u.seasonBarWrapper.Show()
					} else {
						u.seasonBarWrapper.Hide()
					}
				})
				if !running {
					return
				}
			}
		}
	}()
}

func (u *ui) stopSeasonPoll() {
	if u.seasonStop == nil {
		return
	}
	close(u.seasonStop)
	u.seasonStop = nil
}

// ---------------------------------------------------------------------------
// Now Playing screen
// ---------------------------------------------------------------------------

func (u *ui) showNowPlaying() {
	title := widget.NewLabel("")
	pos := widget.NewLabel("0:00 / 0:00")

	seek := widget.NewSlider(0, 1)
	seek.Step = 1
	var seekingDuration float64
	seek.OnChangeEnded = func(v float64) {
		if seekingDuration <= 0 {
			return
		}
		go func() {
			if _, err := u.client.Control("seek", v); err != nil {
				fyne.Do(func() { u.fail(err) })
			}
		}()
	}

	var currentPos float64

	playPause := widget.NewButton("Pause", nil)
	stop := widget.NewButton("Stop", func() {
		go func() {
			if _, err := u.client.Control("stop", 0); err != nil {
				fyne.Do(func() { u.fail(err) })
			}
		}()
	})
	back30 := widget.NewButton("-30", func() { u.seekRelative(currentPos, -30) })
	back10 := widget.NewButton("-10", func() { u.seekRelative(currentPos, -10) })
	fwd10 := widget.NewButton("+10", func() { u.seekRelative(currentPos, 10) })
	fwd30 := widget.NewButton("+30", func() { u.seekRelative(currentPos, 30) })

	vol := widget.NewSlider(0, 1)
	vol.Step = 0.01
	vol.OnChangeEnded = func(v float64) {
		go func() {
			if _, err := u.client.Control("volume", v); err != nil {
				fyne.Do(func() { u.fail(err) })
			}
		}()
	}

	muted := false
	mute := widget.NewButton("Mute", nil)
	mute.OnTapped = func() {
		action := "mute"
		if muted {
			action = "unmute"
		}
		muted = !muted
		if muted {
			mute.SetText("Unmute")
		} else {
			mute.SetText("Mute")
		}
		go func() {
			if _, err := u.client.Control(action, 0); err != nil {
				fyne.Do(func() { u.fail(err) })
			}
		}()
	}

	subtitle := widget.NewButton("Subtitle", func() { u.showSubtitleDialog() })
	back := widget.NewButton("Back to library", func() {
		u.stopNowPlayingPoll()
		u.window.SetContent(u.tabs)
		if u.tabs != nil {
			u.tabs.SelectIndex(0)
		}
	})

	transport := container.NewGridWithColumns(4, back30, back10, fwd10, fwd30)
	controls := container.NewGridWithColumns(2, playPause, stop)

	content := container.NewVBox(
		back,
		title,
		pos,
		seek,
		transport,
		controls,
		widget.NewLabel("Volume"), vol,
		mute,
		subtitle,
	)
	u.window.SetContent(content)

	pauseState := false
	playPause.OnTapped = func() {
		action := "pause"
		if pauseState {
			action = "resume"
		}
		go func() {
			if _, err := u.client.Control(action, 0); err != nil {
				fyne.Do(func() { u.fail(err) })
			}
		}()
	}

	u.stopNowPlayingPoll()
	u.nowPlayingStop = make(chan struct{})
	stopCh := u.nowPlayingStop
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				st, err := u.client.State()
				if err != nil {
					continue
				}
				fyne.Do(func() {
					title.SetText(st.MediaName)
					currentPos = st.CurrentTime
					seekingDuration = st.Duration
					if st.Duration > 0 {
						seek.Max = st.Duration
						seek.Value = st.CurrentTime
						seek.Refresh()
					}
					pos.SetText(fmt.Sprintf("%s / %s", fmtSec(st.CurrentTime), fmtSec(st.Duration)))
					pauseState = st.Status == "paused"
					if pauseState {
						playPause.SetText("Resume")
					} else {
						playPause.SetText("Pause")
					}
					if !muted && st.Muted {
						muted = true
						mute.SetText("Unmute")
					}
					vol.Value = st.Volume
					vol.Refresh()
				})
			}
		}
	}()

}

func (u *ui) showSubtitleDialog() {
	opts := u.currentSubtitleOpts
	if opts.Path == "" {
		opts.Path = u.currentSubtitlePath
	}
	if opts.FontSize == 0 {
		opts.FontSize = 24
	}

	delay := widget.NewSlider(-30, 30)
	delay.Step = 0.5
	delay.Value = opts.DelaySeconds
	delayLbl := widget.NewLabel(fmt.Sprintf("Delay: %.1fs", delay.Value))
	delay.OnChanged = func(v float64) { delayLbl.SetText(fmt.Sprintf("Delay: %.1fs", v)) }

	fs := widget.NewSlider(10, 80)
	fs.Step = 1
	fs.Value = float64(opts.FontSize)
	fsLbl := widget.NewLabel(fmt.Sprintf("Font size: %d", opts.FontSize))
	fs.OnChanged = func(v float64) { fsLbl.SetText(fmt.Sprintf("Font size: %d", int(v))) }

	bold := widget.NewCheck("Bold", nil)
	bold.Checked = opts.Bold
	italic := widget.NewCheck("Italic", nil)
	italic.Checked = opts.Italic
	burn := widget.NewCheck("Burn-in", nil)
	burn.Checked = opts.BurnIn
	ignoreCC := widget.NewCheck("Ignore CC", nil)
	ignoreCC.Checked = opts.IgnoreClosedCaptions

	path := widget.NewEntry()
	path.SetText(opts.Path)

	form := container.NewVBox(
		delayLbl, delay,
		fsLbl, fs,
		bold, italic, burn, ignoreCC,
		widget.NewLabel("Path"), path,
	)
	dialog.ShowCustomConfirm("Subtitle", "Apply", "Cancel", form, func(ok bool) {
		if !ok {
			return
		}
		next := options.SubtitleCastOptions{
			Path:                 path.Text,
			BurnIn:               burn.Checked,
			FontSize:             int(fs.Value),
			IgnoreClosedCaptions: ignoreCC.Checked,
			DelaySeconds:         delay.Value,
			Bold:                 bold.Checked,
			Italic:               italic.Checked,
		}
		u.currentSubtitleOpts = next
		u.currentSubtitlePath = next.Path
		go func() {
			if err := u.client.UpdateSubtitle(next); err != nil {
				fyne.Do(func() { u.fail(err) })
			}
		}()
	}, u.window)
}

func (u *ui) seekRelative(current, delta float64) {
	target := current + delta
	if target < 0 {
		target = 0
	}
	go func() {
		if _, err := u.client.Control("seek", target); err != nil {
			fyne.Do(func() { u.fail(err) })
		}
	}()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func fmtSec(s float64) string {
	if s < 0 {
		s = 0
	}
	total := int(s)
	h := total / 3600
	m := (total % 3600) / 60
	sec := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, sec)
	}
	return fmt.Sprintf("%d:%02d", m, sec)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func fmtSpeed(bps int64) string {
	if bps <= 0 {
		return "0 B/s"
	}
	f := float64(bps)
	switch {
	case f >= 1<<20:
		return fmt.Sprintf("%.1f MB/s", f/(1<<20))
	case f >= 1<<10:
		return fmt.Sprintf("%.1f KB/s", f/(1<<10))
	default:
		return fmt.Sprintf("%d B/s", bps)
	}
}

func fmtEta(sec int64) string {
	if sec <= 0 || sec >= 8640000 {
		return "—"
	}
	m := sec / 60
	s := sec % 60
	return fmt.Sprintf("%dm %ds", m, s)
}
