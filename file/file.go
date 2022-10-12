package file

type File struct {
	Name string `json:"name,omitempty"`
	Hash string `json:"hash,omitempty"`
	Type string `json:"type,omitempty"`
	Ext  string `json:"ext,omitempty"`
	Size int64  `json:"size,omitempty"`
}
