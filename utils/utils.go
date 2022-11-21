package utils

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ulikunitz/xz"
)

type Crdr struct {
	io.Reader
	fin *os.File
}

type TarReader struct {
	*tar.Reader
	Info fs.FileInfo
}

type TarPart struct {
	Writer *tar.Writer
	Reader *TarReader
	Header *tar.Header
}

type JobChannel chan TarPart

const (
	GzipCT string = `application/x-gzip`
	TarCT  string = `application/octet-stream` //an assumption
	XzCT   string = "application/xz-compressed"
)

func GetCTFromFormat(format string) string {
	switch format {
	case "xz":
		return TarCT
	case "none":
		return TarCT
	case "gzip":
		return GzipCT
	default:
		return TarCT
	}
}

func ReadContentType(file *os.File) (ct string, err error) {
	buff := make([]byte, 512)

	if _, err = file.Read(buff); err != nil {
		file.Close()
		return ct, err
	}

	ct = http.DetectContentType(buff)
	if _, err = file.Seek(0, 0); err != nil {
		file.Close()
		return ct, err
	}

	return ct, err
}

func CreateNewOutput(path string) (file *os.File, err error) {
	file, err = os.Create(path)
	return file, err
}

func OpenTarBuffer(file *os.File, format string) (tr *TarReader, rc io.ReadCloser, err error) {
	return GetReader(file, format)
}

func GetSeekedTar(input *os.File, output *os.File, seek int64) (out *os.File, err error) {
	ext := filepath.Ext(input.Name())
	ct, err := ReadContentType(input)
	if err != nil {
		return out, err
	}

	var reader io.Reader
	switch ct {
	case GzipCT:
		reader, err = gzip.NewReader(input)
		if err != nil {
			return out, err
		}
	case TarCT:
		if ext != ".xz" {
			reader = input
			break
		}
		reader, err = xz.NewReader(input)
		if err != nil {
			return out, err
		}
	default:
		reader = input
	}

	if _, err = io.Copy(output, reader); err != nil {
		return out, err
	}

	if err = output.Close(); err != nil {
		return out, err
	}

	out, err = os.OpenFile(output.Name(), os.O_RDWR, os.ModePerm)
	if err != nil {
		return out, err
	}

	seeker := io.SeekStart
	if seek < 0 {
		seeker = io.SeekEnd
	}
	if _, err = out.Seek(seek, seeker); err != nil {
		return out, err
	}

	return out, err
}

func OpenTarFile(pth string) (tr *TarReader, rc io.ReadCloser, err error) {
	var fin *os.File

	if fin, err = os.Open(pth); err != nil {
		return tr, rc, err
	}

	return GetReader(fin, "")
}

func FileExistAndNotEmpty(file *os.File) bool {
	if file == nil {
		return false
	}

	fi, err := file.Stat()
	if err != nil {
		return false
	}

	return fi.Size() > 0
}

func GetReader(fin *os.File, format string) (tr *TarReader, rc io.ReadCloser, err error) {
	if fin == nil {
		err = fmt.Errorf("file input is empty")
		return tr, rc, err
	}

	fi, err := fin.Stat()
	if err != nil {
		fin.Close()
		return tr, rc, err
	}

	ct := ""
	if format != "" {
		ct = GetCTFromFormat(format)
	} else {
		ct, err = ReadContentType(fin)
		if err != nil {
			fin.Close()
			return tr, rc, err
		}
	}

	if rc, err = NewCrdr(fin, ct); err != nil {
		fin.Close()
		err = fmt.Errorf("error opening %s %v", fin.Name(), err)
		return tr, rc, err
	}

	reader := tar.NewReader(rc)
	tr = &TarReader{
		Reader: reader,
		Info:   fi,
	}
	return tr, rc, err
}

func NewCrdr(fin *os.File, contentType string) (c *Crdr, err error) {
	c = &Crdr{
		fin: fin,
	}
	switch contentType {
	case GzipCT:
		c.Reader, err = gzip.NewReader(fin)
	case TarCT:
		c.Reader, err = xz.NewReader(fin)
	default:
		fin.Close()
		err = fmt.Errorf("%s is an unknown file type (unknow %s)", fin.Name(), contentType)
		return c, err
	}

	return c, err
}

func (c *Crdr) Close() (err error) {
	err = c.fin.Close()
	return err
}

func AddTarFromBuffer(tw *tar.Writer, tr *TarReader, rc io.ReadCloser) (err error) {
	var hdr *tar.Header
	for {
		hdr, err = tr.Next()
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}

		part := TarPart{
			Writer: tw,
			Reader: tr,
			Header: hdr,
		}

		err := part.Writer.WriteHeader(part.Header)
		if err != nil {
			break
		}

		_, err = io.Copy(part.Writer, part.Reader)
		if err != nil {
			break
		}
	}

	if err != nil {
		rc.Close()
		return err
	}

	err = rc.Close()

	return err
}

func AddTarFromWriter(tw *tar.Writer, tr *TarReader, rc io.ReadCloser) (err error) {
	var hdr *tar.Header
	// worker := make(JobChannel)
	// result := make(chan error)

	// go func() {
	// 	for {
	// 		select {
	// 		case job := <-worker:
	// 			err = appendPart(job)
	// 			result <- err
	// 		case e := <-result:
	// 			if err != nil {
	// 				err = e
	// 				return
	// 			}
	// 		}
	// 	}
	// }()
	// for {
	// 	hdr, err = tr.Next()
	// 	if err != nil {
	// 		if err == io.EOF {
	// 			err = nil
	// 		}
	// 		break
	// 	}
	// 	job := TarPart{
	// 		Writer: tw,
	// 		Reader: tr,
	// 		Header: hdr,
	// 	}
	// 	worker <- job
	// 	err := <-result
	// 	if err != nil {
	// 		break
	// 	}
	// }

	for {
		hdr, err = tr.Next()
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}

		part := TarPart{
			Writer: tw,
			Reader: tr,
			Header: hdr,
		}

		err = appendPart(part)
		if err != nil {
			break
		}
	}

	if err != nil {
		rc.Close()
		return err
	}

	err = rc.Close()

	return err
}

func appendPart(part TarPart) error {
	err := part.Writer.WriteHeader(part.Header)
	if err != nil {
		return err
	}

	_, err = io.Copy(part.Writer, part.Reader)
	if err != nil {
		return err
	}

	return err
}

func AddTar(tw *tar.Writer, pth string) (err error) {
	var tr *TarReader
	var rc io.ReadCloser

	if tr, rc, err = OpenTarFile(pth); err != nil {
		return err
	}

	return AddTarFromWriter(tw, tr, rc)
}

func AddFileFromBuffer(tw *tar.Writer, file *os.File, fi os.FileInfo, directory string) (err error) {
	err = appendFile(tw, file, fi, directory)
	if err != nil {
		return err
	}

	err = file.Close()
	return err
}

func appendFile(tw *tar.Writer, file *os.File, fi os.FileInfo, directory string) error {
	header, err := tar.FileInfoHeader(fi, directory+fi.Name())
	if err != nil {
		return err
	}

	err = tw.WriteHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(tw, file)
	if err != nil {
		return err
	}

	return err
}

func AddFile(tw *tar.Writer, pth, directory string) (err error) {
	var file *os.File

	fi, err := os.Stat(pth)
	if err != nil {
		return err
	}

	if file, err = os.Open(pth); err != nil {
		return err
	}

	ext := filepath.Ext(pth)

	ct, err := ReadContentType(file)
	if err != nil {
		return err
	}

	switch ct {
	case GzipCT:
		if tr, rc, err := OpenTarFile(pth); err != nil {
			return err
		} else {
			return AddTarFromWriter(tw, tr, rc)
		}
	case TarCT:
		if ext != ".xz" && ext != ".tar" {
			return AddFileFromBuffer(tw, file, fi, directory)
		}
		if tr, rc, err := OpenTarFile(pth); err != nil {
			return err
		} else {
			return AddTarFromWriter(tw, tr, rc)
		}
	default:
		return AddFileFromBuffer(tw, file, fi, directory)
	}
}
