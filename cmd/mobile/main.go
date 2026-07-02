package main

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"wails-cast/pkg/castapi"
)

const (
	prefBaseURL = "baseURL"
	prefToken   = "token"
)

type ui struct {
	fyneApp fyne.App
	window  fyne.Window
	client  *castapi.Client

	devicesMu sync.Mutex
	devices   []castapi.RemoteDevice
	deviceSel *widget.Select
}

func main() {
	a := app.NewWithID("io.wailscast.remote")
	w := a.NewWindow("wails-cast remote")
	w.Resize(fyne.NewSize(420, 720))

	u := &ui{fyneApp: a, window: w}
	u.showConnect()

	w.ShowAndRun()
}

func (u *ui) fail(err error) {
	dialog.ShowError(err, u.window)
}

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
		u.showLibrary()
	}

	discover := widget.NewButton("Discover", func() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			found, err := castapi.Discover(ctx)
			if err != nil {
				u.fail(err)
				return
			}
			instances = found
			list.Refresh()
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
		u.showLibrary()
	})

	form := container.NewVBox(
		widget.NewLabel("Manual connect"),
		host, port, token, connect,
	)

	top := container.NewVBox(
		widget.NewLabel("Discover on LAN"),
		discover,
	)

	content := container.NewBorder(top, form, nil, nil, list)
	u.window.SetContent(content)
}

// ---------------------------------------------------------------------------
// Library screen
// ---------------------------------------------------------------------------

func (u *ui) showLibrary() {
	var items []castapi.LibraryItem

	u.deviceSel = widget.NewSelect([]string{"Local"}, func(string) {})
	u.deviceSel.SetSelectedIndex(0)

	list := widget.NewList(
		func() int { return len(items) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(items[i].Name)
		},
	)
	list.OnSelected = func(i widget.ListItemID) {
		list.Unselect(i)
		if i < 0 || i >= len(items) {
			return
		}
		u.showPlayDialog(items[i])
	}

	refresh := widget.NewButton("Refresh", func() {
		go func() {
			libItems, err := u.client.Library()
			if err != nil {
				u.fail(err)
				return
			}
			items = libItems
			list.Refresh()
		}()
		go u.refreshDevices()
	})

	back := widget.NewButton("Back", func() { u.showConnect() })

	top := container.NewVBox(
		container.NewHBox(back, refresh),
		widget.NewLabel("Output device:"),
		u.deviceSel,
	)

	u.window.SetContent(container.NewBorder(top, nil, nil, nil, list))

	go func() {
		libItems, err := u.client.Library()
		if err != nil {
			u.fail(err)
			return
		}
		items = libItems
		list.Refresh()
	}()
	go u.refreshDevices()
}

func (u *ui) refreshDevices() {
	devs, err := u.client.Devices()
	if err != nil {
		u.fail(err)
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
	u.deviceSel.Options = opts
	u.deviceSel.SetSelectedIndex(0)
	u.deviceSel.Refresh()
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
			u.fail(err)
			return
		}

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
			go func() {
				if _, err := u.client.Play(item.ID, u.selectedDeviceHost(), opts); err != nil {
					u.fail(err)
					return
				}
				u.showNowPlaying()
			}()
		}, u.window)
	}()
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
				u.fail(err)
			}
		}()
	}

	var currentPos float64

	playPause := widget.NewButton("Pause", nil)
	stop := widget.NewButton("Stop", func() {
		go func() {
			if _, err := u.client.Control("stop", 0); err != nil {
				u.fail(err)
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
				u.fail(err)
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
				u.fail(err)
			}
		}()
	}

	back := widget.NewButton("Back to library", func() {
		u.showLibrary()
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
				u.fail(err)
			}
		}()
	}

	stopCh := make(chan struct{})
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

	// Stop the poller if the window is closed.
	u.window.SetOnClosed(func() { close(stopCh) })
}

func (u *ui) seekRelative(current, delta float64) {
	target := current + delta
	if target < 0 {
		target = 0
	}
	go func() {
		if _, err := u.client.Control("seek", target); err != nil {
			u.fail(err)
		}
	}()
}

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
