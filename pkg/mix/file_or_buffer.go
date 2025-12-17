package mix

import "net/http"

type FileOrBuffer struct {
	FilePath string
	Buffer   []byte
	IsBuffer bool
}

func (inp *FileOrBuffer) Serve(w http.ResponseWriter, r *http.Request) {
	if inp.IsBuffer {
		w.Write(inp.Buffer)
	} else {
		http.ServeFile(w, r, inp.FilePath)
	}
}

func (inp *FileOrBuffer) ToPipe() string {
	if inp.IsBuffer {
		return "pipe:0"
	} else {
		return inp.FilePath
	}
}

func File(path string) *FileOrBuffer {
	return &FileOrBuffer{
		FilePath: path,
	}
}

func Buffer(data []byte) *FileOrBuffer {
	return &FileOrBuffer{
		Buffer:   data,
		IsBuffer: true,
	}
}
