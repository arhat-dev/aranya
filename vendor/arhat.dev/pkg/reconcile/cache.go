package reconcile

import "sync"

func NewCache() *Cache {
	return &Cache{
		frozenOldCacheKeys: make(map[interface{}]struct{}),

		oldCache: make(map[interface{}]interface{}),
		cache:    make(map[interface{}]interface{}),

		mu: new(sync.RWMutex),
	}
}

type Cache struct {
	frozenOldCacheKeys map[interface{}]struct{}
	oldCache           map[interface{}]interface{}
	cache              map[interface{}]interface{}

	mu *sync.RWMutex
}

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

		// fill old cache if not initialized
		if _, ok := r.oldCache[key]; !ok {
			r.oldCache[key] = latest
		}
	}
}

func (r *Cache) Delete(key interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.cache, key)
	delete(r.oldCache, key)
}

func (r *Cache) Get(key interface{}) (old, latest interface{}) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.oldCache[key], r.cache[key]
}
