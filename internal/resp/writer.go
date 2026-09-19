package resp

import "io"

// Writer marshals Values and writes them to an underlying io.Writer.
type Writer struct {
	w io.Writer
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

func (w *Writer) Write(v Value) error {
	if v.Multi {
		for _, e := range v.Elems {
			if err := w.Write(e); err != nil {
				return err
			}
		}
		return nil
	}
	_, err := w.w.Write(v.Marshal())
	return err
}
