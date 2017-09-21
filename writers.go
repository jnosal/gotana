package gotana

type recordWriter interface {
	Write(record []string) error
	Flush()
}

func SaveItem(item SaveableItem, writer recordWriter) {
	if writer == nil {
		return
	}

	if !item.Validate() {
		Logger().Warning("Item is not valid. Skipping...")
		return
	}

	defer writer.Flush()
	writer.Write(item.RecordData())
}
