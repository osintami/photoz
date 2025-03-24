// Copyright © 2025 OSINTAMI. This is not yours.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/osintami/photoz/common"
	"github.com/osintami/sloan/log"
)

func main() {

	// handle command line arguments
	var basePath string
	var outPath string
	var clean, debug, stats bool

	flag.StringVar(&basePath, "path", "backups", "starting point")
	flag.StringVar(&outPath, "out", "originals", "output path")
	flag.BoolVar(&clean, "clean", false, "clean logs and db, then run normally")
	flag.BoolVar(&debug, "debug", false, "trace level logging")
	flag.BoolVar(&stats, "stats", false, "existing db stats only")

	flag.Parse()

	// initialize logging interface
	level := "INFO"
	if debug {
		level = "DEBUG"
	}
	log.InitLogger(".", "photoz.log", level, false)

	// initialize file system interface
	fs, err := common.NewFileSystem(basePath)
	if err != nil {
		log.Error().Err(err).Str("photoz", "filesystem").Msg("initialize filesystem failed")
		log.Fatal()
		return
	}

	if stats {
		db, err := common.NewPersistentCache("photoz.db")
		if err != nil && !os.IsNotExist(err) {
			log.Error().Err(err).Str("photoz", "db").Msg("initialize db failed")
			log.Fatal()
			return
		}
		dbStats(db, basePath, outPath, 0)
		return
	}

	if clean {
		err = fs.DeleteFile("photoz.log")
		if err != nil {
			log.Error().Err(err).Str("photoz", "filesystem").Str("file", "photoz.log").Msg("cleanup failure")
		}
		log.InitLogger(".", "photoz.log", level, false)
		fs.DeleteFile("photoz.db")
		if err != nil {
			log.Error().Err(err).Str("photoz", "filesystem").Str("file", "photoz.db").Msg("cleanup failure")
		}
	}

	// initialize duplicates DB
	db, err := common.NewPersistentCache("photoz.db")
	if err != nil && !os.IsNotExist(err) {
		log.Error().Err(err).Str("photoz", "db").Msg("initialize db failed")
		log.Fatal()
		return
	}

	if outPath != "" {
		// TODO: create if necessary, otherwise use ./originals
	}

	fileCount := 0

	// scan recursively for photos
	err = filepath.Walk(basePath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			fileCount += 1
			toIgnoreByName, _ := fs.IgnoreByName(filePath)
			if toIgnoreByName {
				log.Debug().Str("photoz", "file").Str("file", filePath).Msg("skip by name")
				return nil
			}

			toIgnoreByExt, extension := fs.IgnoreByExtension(filePath)
			if toIgnoreByExt {
				log.Debug().Str("photoz", "file").Str("file", filePath).Str("ext", extension).Msg("skip by extension")
				return nil
			}

			isImg, mimeType, err := fs.IsImage(filePath)
			if err != nil {
				log.Error().Str("photoz", "file").Str("file", filePath).Msg("mime type failed")
			} else if isImg {
				log.Debug().Str("photoz", "file").Str("file", filePath).Str("type", mimeType).Msg("processing")
				// get image md5
				md5, err := fs.CalculateMD5(filePath)
				if err != nil {
					log.Error().Err(err).Str("photoz", "file").Str("file", filePath).Msg("md5 failure")
					return nil
				}
				// check db for duplicate
				fi := common.ImageFileInfo{}
				obj, found := db.Get(md5, fi)
				if found {
					fi := obj.(common.ImageFileInfo)
					// log.Info().Str("photoz", "file").Str("file", filePath).Msg("duplicate")
					fi.Duplicates++
					db.Set(md5, fi, -1)
					return nil
				} else {
					fi := common.NewImageFileInfo(filePath, mimeType, md5)
					log.Debug().Str("photoz", "file").Str("file", filePath).Msg("original")
					outFile := ""
					if fi.IsJPEG() || fi.IsNEF() || fi.IsHEIC() {
						// parse the EXIF data
						err := fi.GetJpegCreatedAt()
						if err == nil {
							fi.SetFileName()
							fi.HasExif = true
							outFile = fi.FileName
						} else {
							if fi.IsNEF() {
								log.Fatal().Msg("NEF with no EXIF data!!!")
							}
							outFile = md5 + "_" + filepath.Base(filePath)
						}
					} else {
						// create output file name using md5+original name
						outFile = md5 + "_" + filepath.Base(filePath)
					}

					// sync object changes back to the db
					db.Set(md5, fi, -1)

					// copy to output directory
					log.Debug().Msg("cp " + filePath + " , " + outPath + "/" + outFile)
					err := fs.CopyFile(filePath, outPath+"/"+outFile)
					if err != nil {
						log.Error().Err(err).Str("photoz", "copy").Str("inFile", filePath).Str("outFile", outPath+"/"+outFile).Msg("original file copy failed")
					}
				}

				return nil
			}

		} else {
			log.Debug().Str("photoz", "file").Str("file", filePath).Msg("unrecognized file")
		}

		return nil
	})

	if err != nil {
		log.Error().Err(err).Str("photoz", "file").Msg("directory traverse failed")
	}

	// save the results
	err = db.Persist()
	if err != nil {
		log.Error().Err(err).Str("photoz", "db").Msg("persisting duplicate photo db")
	}
	dbStats(db, basePath, outPath, fileCount)

}

func dbStats(db *common.FastCache, basePath, outPath string, fileCount int) {
	// print stats
	jsonList := db.List()
	itemList := make([]common.ImageFileInfo, 0)
	for _, jsonString := range jsonList {
		obj := common.ImageFileInfo{}
		//fmt.Println(jsonString)
		json.Unmarshal([]byte(jsonString), &obj)
		itemList = append(itemList, obj)
	}

	var dups, jpeg, tif, gif, nef, exif, bmp, png, rtf, avi, heic, mjpeg, totalImages int32
	for _, item := range itemList {
		dups += item.Duplicates
		if item.MimeType == "image/jpeg" {
			jpeg += 1
		} else if item.MimeType == "image/heic" {
			heic += 1
		} else if item.MimeType == "image/nef" {
			nef += 1
		} else if item.MimeType == "image/gif" {
			gif += 1
		} else if item.MimeType == "image/tiff" {
			tif += 1
		} else if item.MimeType == "image/png" {
			png += 1
		} else if item.MimeType == "image/bmp" {
			bmp += 1
		} else if item.MimeType == "application/rtf" {
			rtf += 1
		} else if item.MimeType == "video/x-msvideo" {
			avi += 1
		} else if item.MimeType == "video/mjpeg" {
			mjpeg += 1
		}
		if item.HasExif {
			exif += 1
		}
	}
	totalImages = int32(len(itemList))
	// TODO:  write to log file properly for reporting
	fmt.Println("     INPUT: ", basePath)
	fmt.Println("    OUTPUT: ", outPath)
	fmt.Println(" PROCESSED: ", fileCount)
	fmt.Println("DUPLICATES: ", dups)
	fmt.Println("    IMAGES: ", totalImages)
	fmt.Println("      JPEG: ", jpeg)
	fmt.Println("       NEF: ", nef)
	fmt.Println("      EXIF: ", exif)
	fmt.Println("      HEIC: ", heic)
	fmt.Println("       GIF: ", gif)
	fmt.Println("      TIFF: ", tif)
	fmt.Println("       BMP: ", bmp)
	fmt.Println("       PNG: ", png)
	fmt.Println("       RTF: ", rtf)
	fmt.Println("       AVI: ", avi)
	fmt.Println("     MJPEG: ", mjpeg)

	if (jpeg + nef + heic + gif + tif + bmp + png + rtf + avi + mjpeg) != totalImages {
		fmt.Println("WARNING:  Total Images != (JPEG + NEF + HEIC + GIF + TIFF + BMP + PNG + RTF + AVI + MJPEG)")
	}
	if (jpeg + nef) != exif {
		fmt.Println("WARNING:  JPEG/NEF images with missing EXIF data detected")
	}
}
