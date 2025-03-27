// Copyright © 2025 OSINTAMI. This is not yours.
package common

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/osintami/sloan/log"
)

type FileSystem struct {
	BasePath string
}

var skipExtensions = map[string]string{
	".html":     "html",
	".htm":      "htm",
	".doc":      "doc",
	".db":       "db",
	".jbf":      "jbf",
	".dot":      "dot",
	".txt":      "txt",
	".class":    "class",
	".pdd":      "pdd",
	".xls":      "xls",
	".eml":      "eml",
	".wab":      "wab",
	".ptn":      "ptn",
	".dmf":      "dmf",
	".log":      "log",
	".ds_store": "ds_store",
	".ps":       "ps",
	".mp3":      "mp3",
	".svn-base": "svn",
	".gz":       "gzip",
	".java":     "java",
	".xml":      "xml",
	".jar":      "jar",
	".test":     "test",
	".css":      "css",
	".exe":      "exe",
	".deb":      "deb",
	".zip":      "zip",
	".lisj":     "lisj",
	".lij":      "lij",
	".plist":    "plist",
	".bak":      "bak",
	".wav":      "wav",
	".xmp":      "xmp",
	".rtf":      "rtf",
	".json":     "json",
	// consider re-enabling once we fix other issues
	"mjpg": "mjpg",
}

var imageSignatures = map[string]string{
	"ftypisom":                         "video/mp4",       // MPEG4
	"ftypMSNV":                         "video/mp4",       // MPEG4
	"\xff\xd8\xff":                     "image/jpeg",      // JPEG
	"GIF87a":                           "image/gif",       // GIF
	"GIF89a":                           "image/gif",       // GIF
	"BM":                               "image/bmp",       // BMP
	"II*\x00":                          "image/tiff",      // TIFF (little-endian)
	"MM\x00*":                          "image/tiff",      // TIFF (big-endian)
	"RIFF....WEBP":                     "image/webp",      // WEBP
	"\x52\x49\x46\x46":                 "video/x-msvideo", // AVI
	"\x7B\x5C\x72\x74\x66\x31":         "application/rtf", // RTF
	"\x49\x44\x33":                     "audio/mpeg",      // MP3
	"\x00\x00\x00\x28ftypheic":         "image/heic",      // HEIC
	"\x89\x50\x4E\x47\x0D\x0A\x1A\x0A": "image/png",       // PNG or NEF WTF???
	// consider re-enabling once we fix other issues
	//"\x0D\x0A\x0D\x0A\x2D\x2D\x6D\x79\x62\x6F\x75\x6E\x64\x61\x72\x79": "video/mjpeg", // MJPEG
}

func NewFileSystem(basePath string) (*FileSystem, error) {
	_, err := os.Stat(basePath)
	if os.IsNotExist(err) {
		log.Error().Err(err).Str("photoz", "filesystem").Str("file", basePath).Msg("does not exist")
		return nil, err
	}
	return &FileSystem{BasePath: basePath}, nil
}

func (x *FileSystem) IgnoreByName(filePath string) (bool, string) {
	name := filepath.Base(filePath)
	// Apple metadata files which appear to be good image files, but aren't
	if strings.HasPrefix(name, "._") {
		return true, name
	}
	return false, ""
}

func (x *FileSystem) IgnoreByExtension(filePath string) (bool, string) {
	suffix := filepath.Ext(filePath)
	for ext, name := range skipExtensions {

		if strings.EqualFold(suffix, ext) {
			return true, name
		}
	}
	return false, ""
}

func (x *FileSystem) IsImage(filePath string) (bool, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, "", err
	}
	defer file.Close()

	buffer := make([]byte, 32)
	_, err = io.ReadFull(file, buffer)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return false, "", err
	}

	for magic, mime := range imageSignatures {
		if bytes.HasPrefix(buffer, []byte(magic)) {
			// HACK ALERT:  the PNG and NEF files share the same magic number GRRRR...
			if mime == "image/png" {
				suffix := filepath.Ext(filePath)
				isNEF := strings.EqualFold(suffix, ".NEF")
				if isNEF {
					mime = "image/nef"
				}
			}

			return true, mime, nil
		}
	}

	return false, "", nil
}

func (x *FileSystem) CalculateMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Error().Err(err).Str("photoz", "md5").Msg("file open failed")
		return "", err
	}
	defer file.Close()

	hash := md5.New()

	if _, err := io.Copy(hash, file); err != nil {
		log.Error().Err(err).Str("photoz", "md5").Msg("copy bytes failed")
		return "", err
	}

	hashInBytes := hash.Sum(nil)
	return hex.EncodeToString(hashInBytes), nil
}

func (x *FileSystem) CopyFile(inFile, outFile string) error {
	src, err := os.Open(inFile)
	if err != nil {
		log.Error().Err(err).Str("component", "filesystem").Str("file", outFile).Msg("open")
		return err
	}
	defer src.Close()

	dst, err := os.Create(outFile)
	if err != nil {
		log.Error().Err(err).Str("component", "filesystem").Str("file", outFile).Msg("create")
		return err
	}
	defer dst.Close()

	written, err := io.Copy(dst, src)
	if err != nil || written == 0 {
		log.Error().Err(err).Str("component", "filesystem").Str("file", outFile).Msg("copy")
		if err == nil {
			err = errors.New("no bytes copied")
		}
		return err
	}

	return x.Chmod(outFile, 0644)
}

func (x *FileSystem) DeleteFile(inFile string) error {
	err := os.Remove(inFile)
	if err != nil {
		log.Error().Err(err).Str("component", "filesystem").Str("file", inFile).Msg("delete")
		return err
	}
	return nil
}

func (x *FileSystem) Chmod(inFile string, mode fs.FileMode) error {
	err := os.Chmod(inFile, 0644)
	if err != nil {
		log.Error().Err(err).Str("component", "filesystem").Str("file", inFile).Msg("chmod")
		return err
	}
	return nil
}
