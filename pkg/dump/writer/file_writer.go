package writer

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/DataDog/KubeHound/pkg/telemetry/log"
	"github.com/DataDog/KubeHound/pkg/telemetry/span"
	"github.com/DataDog/KubeHound/pkg/telemetry/tag"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

const (
	FileWriterChmod = 0600
	FileTypeTag     = "file"

	// Multi-threading the dump with one worker for each types
	// The number of workers is set to the number of differents entities (roles, pods, ...)
	// 1 thread per k8s object type to pull from the Kubernetes API
	// 0 means as many thread as k8s entity types (calculated by the dumper_pipeline)
	FileWriterWorkerNumber = 0
)

// The FileWriter uses a map of buffers to write data to files
// Each file has its own buffer to optimize IO calls
type FileWriter struct {
	buffers         map[string]*bufio.Writer
	files           map[string]*os.File
	directoryOutput string
}

func NewFileWriter(ctx context.Context, directoryOutput string, resName string) (*FileWriter, error) {
	return &FileWriter{
		buffers:         make(map[string]*bufio.Writer),
		files:           make(map[string]*os.File),
		directoryOutput: path.Join(directoryOutput, resName),
	}, nil
}

func (f *FileWriter) OutputPath() string {
	return f.directoryOutput
}

func (f *FileWriter) WorkerNumber() int {
	return FileWriterWorkerNumber
}

func (f *FileWriter) Write(ctx context.Context, data []byte, filePath string) error {
	span, _ := tracer.StartSpanFromContext(ctx, span.DumperWriterWrite, tracer.Measured())
	span.SetTag(tag.DumperFilePathTag, filePath)
	span.SetTag(tag.DumperWriterTypeTag, FileTypeTag)
	defer span.Finish()
	filePath = path.Join(f.directoryOutput, filePath)

	buffer, ok := f.buffers[filePath]
	if !ok {
		// Create directories if they do not exist
		err := os.MkdirAll(filepath.Dir(filePath), WriterDirChmod)
		if err != nil {
			return fmt.Errorf("failed to create directories: %w", err)
		}

		file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, FileWriterChmod)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		f.files[filePath] = file
		buffer = bufio.NewWriter(file)
		f.buffers[filePath] = buffer
	}
	_, err := buffer.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write JSON data to buffer: %w", err)
	}

	return nil
}

func (f *FileWriter) Flush(ctx context.Context) error {
	log.I.Debug("Flushing writers")
	span, _ := tracer.StartSpanFromContext(ctx, span.DumperWriterFlush, tracer.Measured())
	span.SetTag(tag.DumperWriterTypeTag, FileTypeTag)
	defer span.Finish()
	for path, writer := range f.buffers {
		err := writer.Flush()
		if err != nil {
			return fmt.Errorf("failed to flush writer: %w", err)
		}
		delete(f.buffers, path)
	}
	f.buffers = make(map[string]*bufio.Writer)

	return nil
}

func (f *FileWriter) Close(ctx context.Context) error {
	log.I.Debug("Closing writers")
	span, _ := tracer.StartSpanFromContext(ctx, span.DumperWriterClose, tracer.Measured())
	span.SetTag(tag.DumperWriterTypeTag, FileTypeTag)
	var err error
	defer func() { span.Finish(tracer.WithError(err)) }()
	for path, file := range f.files {
		err := file.Close()
		if err != nil {
			return fmt.Errorf("failed to close writer: %w", err)
		}
		delete(f.files, path)
	}

	f.files = make(map[string]*os.File)

	return nil
}
