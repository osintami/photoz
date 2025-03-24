// Copyright © 2023 OSINTAMI. This is not yours.
package common

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/osintami/sloan/log"
	"github.com/patrickmn/go-cache"
)

type IFastCache interface {
	Get(key string, obj interface{}) (interface{}, bool)
	Set(key string, value interface{}, duration time.Duration)
	Clear()
	Persist() error
	ToJSON(string) error
	toJson(interface{})
	fromJson(interface{})
}

type FastCache struct {
	persistFile string
	cache       *cache.Cache
}

func NewFastCache() *FastCache {
	return &FastCache{cache: cache.New(24*time.Hour, 60*time.Minute)}
}

func NewPersistentCache(persistFile string) (*FastCache, error) {
	x := &FastCache{
		persistFile: persistFile,
		cache:       cache.New(24*time.Hour, 60*time.Minute)}
	return x, x.cache.LoadFile(persistFile)
}

func (x *FastCache) Get(key string, obj ImageFileInfo) (interface{}, bool) {
	jsonString, found := x.cache.Get(key)
	if found {
		obj, err := x.fromJSON(jsonString.(string), obj)
		if err != nil {
			log.Error().Err(err).Str("fastcache", "get").Msg("toJson")
			return nil, false
		}
		return obj, true
	}
	return nil, false
}

func (x *FastCache) Set(key string, value interface{}, duration time.Duration) {
	jsonString, err := x.toJSON(value)
	if err != nil {
		log.Error().Err(err).Str("fastcache", "set").Msg("fromJson")
		return
	}
	x.cache.Set(key, jsonString, duration)
}

func (x *FastCache) LoadFile(fileName string) *FastCache {
	x.cache.LoadFile(fileName)
	return x
}

func (x *FastCache) Save(fileName string) error {
	return x.cache.SaveFile(fileName)
}

func (x *FastCache) Persist() error {
	return x.cache.SaveFile(x.persistFile)
}

func (x *FastCache) Clear() {
	for k := range x.cache.Items() {
		x.cache.Delete(k)
	}
}

func (x *FastCache) Delete(pattern string) {
	for k := range x.cache.Items() {
		if strings.Contains(k, pattern) {
			x.cache.Delete(k)
		}
	}
}

func (x *FastCache) List() []string {
	out := make([]string, 0)
	for _, v := range x.cache.Items() {
		out = append(out, v.Object.(string))
	}
	return out
}

func (x *FastCache) ToJSON(fileName string) error {
	out := make([]interface{}, 0)
	for _, v := range x.cache.Items() {
		out = append(out, v.Object)
	}
	json, _ := json.MarshalIndent(out, "", "    ")
	return os.WriteFile(fileName, []byte(json), 0644)
}

func (x *FastCache) toJSON(fi interface{}) (string, error) {
	jsonData, err := json.Marshal(fi)
	if err != nil {
		log.Error().Err(err).Str("photoz", "json").Msg("marshall JSON")
		return "", err
	}
	return string(jsonData), nil
}

func (x *FastCache) fromJSON(jsonString string, obj ImageFileInfo) (interface{}, error) {
	err := json.Unmarshal([]byte(jsonString), &obj)
	return obj, err
}
