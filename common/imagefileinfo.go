// Copyright © 2025 OSINTAMI. This is not yours.
package common

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/dsoprea/go-exif/v3"
	"github.com/osintami/sloan/log"
)

type ImageFileInfo struct {
	FilePath         string `json:"filepath"`
	MimeType         string `json:"mimetype"`
	MD5              string `json:"md5"`
	FileName         string `json:"filename"`
	OriginalDateTime string `json:"originaldatetime"`
	Duplicates       int32  `json:"duplicates"`
	HasExif          bool   `json:"hasexif"`
}

func NewImageFileInfo(filePath, mimeType, md5 string) ImageFileInfo {
	ifi := ImageFileInfo{}
	ifi.FilePath = filePath
	ifi.MimeType = mimeType
	ifi.MD5 = md5
	return ifi
}

func (x *ImageFileInfo) GetJpegCreatedAt() error {
	// extract the EXIF data from a file
	rawExif, err := exif.SearchFileAndExtractExif(x.FilePath)
	if err != nil {
		log.Error().Err(err).Str("photoz", "exif").Str("file", x.FilePath).Msg("exif extract")
		return err
	}

	// parse the raw EXIF data into a structured format
	tags, _, err := exif.GetFlatExifData(rawExif, nil)
	if err != nil {
		log.Error().Err(err).Str("photoz", "exif").Str("file", x.FilePath).Msg("exif parse")
		return err
	}

	if false {
		for _, tag := range tags {
			fmt.Printf("Tag: %s, Value: %v\n", tag.TagName, tag.Value)
		}
	}

	originalTime := ""

	for _, tag := range tags {
		// JPEG and NEF tag names for original date
		if tag.TagName == "DateTimeOriginal" || tag.TagName == "Create Date" {
			originalTime = fmt.Sprintf("%v", tag.Value)
		}
	}

	date, err := time.Parse("2006:01:02 15:04:05", originalTime)
	if err != nil {
		log.Error().Err(err).Str("photoz", "exif").Str("file", x.FilePath).Msg("time parse")
		return err
	}

	originalTime = date.Format(time.RFC3339)

	x.OriginalDateTime = originalTime
	return nil
}

func (x *ImageFileInfo) SetFileName() {
	if x.OriginalDateTime != "" {
		x.FileName = x.OriginalDateTime + "_" + x.MD5 + "_" + filepath.Base(x.FilePath)
	} else {
		x.FileName = "unknown" + "_" + x.MD5 + "_" + filepath.Base(x.FilePath)
	}
}

func (x *ImageFileInfo) IsJPEG() bool {
	return x.MimeType == "image/jpeg"
}

func (x *ImageFileInfo) IsNEF() bool {
	suffix := filepath.Ext(x.FilePath)
	isNEF := strings.EqualFold(suffix, ".NEF")
	if isNEF {
		x.MimeType = "image/nef"
	}
	return isNEF
}

func (x *ImageFileInfo) IsHEIC() bool {
	suffix := filepath.Ext(x.FilePath)
	isNEF := strings.EqualFold(suffix, ".HEIC")
	if isNEF {
		x.MimeType = "image/heic"
	}
	return isNEF
}
