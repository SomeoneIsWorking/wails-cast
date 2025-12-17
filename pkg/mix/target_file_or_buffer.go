package mix

type TargetFileOrBuffer struct {
	FilePath string
	IsBuffer bool
}

func (out *TargetFileOrBuffer) ToOutput() *FileOrBuffer {
	return File(out.FilePath)
}

func (out *TargetFileOrBuffer) ToPipe() string {
	if out.IsBuffer {
		return "pipe:1"
	} else {
		return out.FilePath
	}
}

func FileTarget(path string) *TargetFileOrBuffer {
	return &TargetFileOrBuffer{
		FilePath: path,
	}
}

func BufferTarget() *TargetFileOrBuffer {
	return &TargetFileOrBuffer{
		IsBuffer: true,
	}
}
