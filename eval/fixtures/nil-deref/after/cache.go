package cache

type Cache struct{ data map[string]string }

func New() *Cache {
	return &Cache{data: map[string]string{}}
}

func (c *Cache) Get(key string) string {
	return c.data[key]
}

func First(cs []*Cache) string {
	return cs[0].data["first"]
}
