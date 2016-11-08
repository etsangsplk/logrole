// Package cache caches Twilio API requests for fast loading.
//
// Fetching a second page of resources from Twilio can be extremely slow - one
// second or more. Often we know the URL we want to fetch in advance - the
// first page of Messages or Calls, and any next_page_uri as soon as a user
// retrieves any individual page. Fetching the page and caching it can greatly
// improve latency.
package cache

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"errors"
	"sync"
	"time"

	"github.com/golang/groupcache/lru"
	log "github.com/inconshreveable/log15"
)

type Cache struct {
	log.Logger
	c  *lru.Cache
	mu sync.RWMutex
}

var expired = errors.New("expired")
var errNotFound = errors.New("Key not found in cache")

func NewCache(size int, l log.Logger) *Cache {
	return &Cache{
		Logger: l,
		c:      lru.New(size),
	}
}

// enc gob.Encodes + gzips data. do not try to gob.Encode an interface
func enc(data interface{}) []byte {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	ec := gob.NewEncoder(writer)
	if err := ec.Encode(data); err != nil {
		panic(err)
	}
	if err := writer.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// Get gets the value at the key and decodes it into val. Returns the time the
// value was stored in the cache, or an error, if the value was not found,
// expired, or could not be decoded into val.
func (c *Cache) Get(key string, val interface{}) (time.Time, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cacheVal, ok := c.c.Get(key)
	if !ok {
		c.Debug("cache miss", "key", key)
		return time.Time{}, errNotFound
	}
	e, ok := cacheVal.(*expiringBits)
	if !ok {
		c.Warn("Invalid value in cache", "val", cacheVal, "key", key)
		return time.Time{}, errors.New("could not cast value to expiringBits")
	}
	if since := time.Since(e.Expires); since > 0 {
		c.Debug("found expired value in cache", "key", key, "expired_ago", since)
		c.c.Remove(key)
		return time.Time{}, expired
	}
	reader, err := gzip.NewReader(bytes.NewReader(e.Bits))
	if err != nil {
		panic(err)
	}
	defer reader.Close()
	dec := gob.NewDecoder(reader)
	if err := dec.Decode(val); err != nil {
		return time.Time{}, err
	}
	c.Debug("cache hit", "key", key, "size", len(e.Bits))
	return e.Set, nil
}

func (c *Cache) Set(key string, val interface{}, timeout time.Duration) {
	now := time.Now().UTC()
	c.mu.Lock()
	defer c.mu.Unlock()
	e := &expiringBits{
		Set:     now,
		Expires: now.Add(timeout),
		Bits:    enc(val),
	}
	c.c.Add(key, e)
	c.Debug("stored data in cache", "key", key, "size", len(e.Bits), "cache_size", c.c.Len())
}

type expiringBits struct {
	Set     time.Time
	Expires time.Time
	Bits    []byte // call enc() to get an encoded value
}
