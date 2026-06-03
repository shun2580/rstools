package remotestorage

import "strings"

// WalkDir recursively lists all files under path, returning paths relative to path.
func (c *Client) WalkDir(path string) ([]Entry, error) {
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	entries, err := c.ListDir(path)
	if err != nil {
		return nil, err
	}
	var result []Entry
	for _, e := range entries {
		if e.IsDir {
			sub, err := c.WalkDir(path + e.Name)
			if err != nil {
				return nil, err
			}
			for i := range sub {
				sub[i].Name = e.Name + sub[i].Name
			}
			result = append(result, sub...)
		} else {
			result = append(result, e)
		}
	}
	return result, nil
}
