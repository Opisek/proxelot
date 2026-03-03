package dialog

type Dialog struct {
	Title       string
	Description string
}

func (dialog *Dialog) SerializeDialog() []byte {
	return serializeNbt(&struct {
		Type      string
		Title     string
		Body      []any
		Closeable uint8 `nbt:"can_close_with_escape"`
		Columns   uint32
	}{
		Type:  "minecraft:notice",
		Title: dialog.Title,
		Body: []any{
			struct {
				Type string
				Item any
			}{
				Type: "minecraft:item",
				Item: struct {
					Id string
				}{
					Id: "minecraft:clock",
				},
			},
			struct {
				Type     string
				Contents string
			}{
				Type:     "minecraft:plain_message",
				Contents: dialog.Description,
			},
		},
		Closeable: 0,
		Columns:   0,
	})
}
