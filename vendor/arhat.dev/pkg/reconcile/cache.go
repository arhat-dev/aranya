/*
Copyright 2020 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package reconcile

import "sync"

func NewCache() *Cache {
	return &Cache{
		frozenOldCacheKeys: make(map[interface{}]struct{}),

		cache:    make(map[interface{}]interface{}),
		oldCache: make(map[interface{}]interface{}),

		mu: new(sync.RWMutex),
	}
}

type Cache struct {
	frozenOldCacheKeys map[interface{}]struct{}

	cache    map[interface{}]interface{}
	oldCache map[interface{}]interface{}

	mu *sync.RWMutex
}

// Freeze object with key in old cache
func (r *Cache) Freeze(key interface{}, freeze bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if freeze {
		r.frozenOldCacheKeys[key] = struct{}{}
	} else {
		delete(r.frozenOldCacheKeys, key)
	}
}

func (r *Cache) Update(key interface{}, old, latest interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, frozen := r.frozenOldCacheKeys[key]; !frozen {
		if old != nil {
			r.oldCache[key] = old
		} else if o, ok := r.cache[key]; ok {
			// move cached to old cached
			r.oldCache[key] = o
		}
	}

	if latest != nil {
		r.cache[key] = latest

		// fill old cache if not initialized regardless whether it is frozen
		if _, ok := r.oldCache[key]; !ok {
			r.oldCache[key] = latest
		}
	}
}

func (r *Cache) Delete(key interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.frozenOldCacheKeys, key)
	delete(r.cache, key)
	delete(r.oldCache, key)
}

func (r *Cache) Get(key interface{}) (old, latest interface{}) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.oldCache[key], r.cache[key]
}
