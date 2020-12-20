package output

import (
	"bytes"
	"container/list"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"miller/clitypes"
	"miller/types"
)

// ostream *os.File in constructors/factory
type RecordWriterPPRINT struct {
	// Input:
	records *list.List
	// For detecting schema changes: we print a newline and the new header.
	barred bool

	// State:
	lastJoinedHeader *string
	batch            *list.List
}

func NewRecordWriterPPRINT(writerOptions *clitypes.TWriterOptions) *RecordWriterPPRINT {
	return &RecordWriterPPRINT{
		records: list.New(),
		barred:  writerOptions.BarredPprintOutput,

		lastJoinedHeader: nil,
		batch:            list.New(),
	}
}

// ----------------------------------------------------------------
// TODO this is very naive at present -- needs copy from the C version.
func (this *RecordWriterPPRINT) Write(
	outrec *types.Mlrmap,
) {
	// Group records by have-same-schema or not. Pretty-print each
	// homoegeneous sublist, or "batch".
	//
	// No output until end of a homogeneous batch of records, since we need to
	// find out max width down each column.

	if outrec != nil { // Not end of record stream

		if this.lastJoinedHeader == nil {
			// First output record:
			// * New batch
			// * No old batch to print
			this.batch.PushBack(outrec)
			temp := strings.Join(outrec.GetKeys(), ",")
			this.lastJoinedHeader = &temp
		} else {
			// May or may not continue the same homogeneous batch
			joinedHeader := strings.Join(outrec.GetKeys(), ",")
			if *this.lastJoinedHeader != joinedHeader {
				// Print and free old batch
				this.writeHeterogenousList(this.batch, this.barred)
				// Print a newline
				// TODO: fix me -- don't write final extra newline at end of stream
				os.Stdout.WriteString("\n")
				// Start a new batch
				this.batch = list.New()
				this.batch.PushBack(outrec)
				this.lastJoinedHeader = &joinedHeader
			} else {
				// Continue the batch
				this.batch.PushBack(outrec)
			}
		}

	} else { // End of record stream

		if this.batch.Front() != nil {
			this.writeHeterogenousList(this.batch, this.barred)
		}
	}
}

// ----------------------------------------------------------------
func (this *RecordWriterPPRINT) writeHeterogenousList(
	records *list.List,
	barred bool,
) {
	maxWidths := make(map[string]int)

	for e := records.Front(); e != nil; e = e.Next() {
		outrec := e.Value.(*types.Mlrmap)
		for pe := outrec.Head; pe != nil; pe = pe.Next {
			width := utf8.RuneCountInString(pe.Value.String())
			if width == 0 {
				width = 1 // We'll rewrite "" to "-" below
			}
			oldMaxWidth := maxWidths[*pe.Key]
			if width > oldMaxWidth {
				maxWidths[*pe.Key] = width
			}
		}
	}

	// Column name may be longer/shorter than all data values in the column
	for key, oldMaxWidth := range maxWidths {
		width := utf8.RuneCountInString(key)
		if width > oldMaxWidth {
			maxWidths[key] = width
		}
	}
	if barred {
		this.writeHeterogenousListBarred(records, maxWidths)
	} else {
		this.writeHeterogenousListNonBarred(records, maxWidths)
	}
}

// ----------------------------------------------------------------
// Example:
//
// a   b   i  x                    y
// pan pan 1  0.3467901443380824   0.7268028627434533
// eks pan 2  -0.7586799647899636  0.5221511083334797
// wye wye 3  0.20460330576630303  0.33831852551664776
// eks wye 4  -0.38139939387114097 0.13418874328430463
// wye pan 5  0.5732889198020006   0.8636244699032729

func (this *RecordWriterPPRINT) writeHeterogenousListNonBarred(
	records *list.List,
	maxWidths map[string]int,
) {

	onFirst := true
	for e := records.Front(); e != nil; e = e.Next() {
		outrec := e.Value.(*types.Mlrmap)

		// Print header line
		if onFirst {
			var buffer bytes.Buffer // faster than fmt.Print() separately
			for pe := outrec.Head; pe != nil; pe = pe.Next {
				if pe.Next != nil {
					buffer.WriteString(fmt.Sprintf("%-*s ", maxWidths[*pe.Key], *pe.Key))
				} else {
					buffer.WriteString(*pe.Key)
					buffer.WriteString("\n") // TODO: ORS
				}
			}
			os.Stdout.WriteString(buffer.String())
		}
		onFirst = false

		// Print data lines
		var buffer bytes.Buffer // faster than fmt.Print() separately
		for pe := outrec.Head; pe != nil; pe = pe.Next {
			s := pe.Value.String()
			if s == "" {
				s = "-"
			}
			if pe.Next != nil {
				buffer.WriteString(fmt.Sprintf("%-*s ", maxWidths[*pe.Key], s))
			} else {
				buffer.WriteString(s)
				buffer.WriteString("\n") // TODO: ORS
			}
		}
		os.Stdout.WriteString(buffer.String())
	}
}

// ----------------------------------------------------------------
// Example:
//
// +-----+-----+----+----------------------+---------------------+
// | a   | b   | i  | x                    | y                   |
// +-----+-----+----+----------------------+---------------------+
// | pan | pan | 1  | 0.3467901443380824   | 0.7268028627434533  |
// | eks | pan | 2  | -0.7586799647899636  | 0.5221511083334797  |
// | wye | wye | 3  | 0.20460330576630303  | 0.33831852551664776 |
// | eks | wye | 4  | -0.38139939387114097 | 0.13418874328430463 |
// | wye | pan | 5  | 0.5732889198020006   | 0.8636244699032729  |
// +-----+-----+----+----------------------+---------------------+

// TODO: for better performance, uuse string-buffer as in DKVP for this and all
// record-writers

func (this *RecordWriterPPRINT) writeHeterogenousListBarred(
	records *list.List,
	maxWidths map[string]int,
) {

	horizontalBars := make(map[string]string)
	for key, width := range maxWidths {
		horizontalBars[key] = strings.Repeat("-", width)
	}
	horizontalStart := "+-"
	horizontalMiddle := "-+-"
	horizontalEnd := "-+"
	verticalStart := "| "
	verticalMiddle := " | "
	verticalEnd := " |"

	onFirst := true
	for e := records.Front(); e != nil; e = e.Next() {
		outrec := e.Value.(*types.Mlrmap)

		// Print header line
		if onFirst {
			var buffer bytes.Buffer // faster than fmt.Print() separately

			buffer.WriteString(horizontalStart)
			for pe := outrec.Head; pe != nil; pe = pe.Next {
				buffer.WriteString(horizontalBars[*pe.Key])
				if pe.Next != nil {
					buffer.WriteString(horizontalMiddle)
				} else {
					buffer.WriteString(horizontalEnd)
					buffer.WriteString("\n") // TOOD: ORS
				}
			}

			buffer.WriteString(verticalStart)
			for pe := outrec.Head; pe != nil; pe = pe.Next {
				buffer.WriteString(fmt.Sprintf("%-*s", maxWidths[*pe.Key], *pe.Key))
				if pe.Next != nil {
					buffer.WriteString(verticalMiddle)
				} else {
					buffer.WriteString(verticalEnd)
					buffer.WriteString("\n") // TOOD: ORS
				}
			}

			buffer.WriteString(horizontalStart)
			for pe := outrec.Head; pe != nil; pe = pe.Next {
				buffer.WriteString(horizontalBars[*pe.Key])
				if pe.Next != nil {
					buffer.WriteString(horizontalMiddle)
				} else {
					buffer.WriteString(horizontalEnd)
					buffer.WriteString("\n") // TOOD: ORS
				}
			}

			os.Stdout.WriteString(buffer.String())
		}
		onFirst = false

		// Print data lines
		var buffer bytes.Buffer // faster than fmt.Print() separately
		buffer.WriteString(verticalStart)
		for pe := outrec.Head; pe != nil; pe = pe.Next {
			s := pe.Value.String()
			buffer.WriteString(fmt.Sprintf("%-*s", maxWidths[*pe.Key], s))
			if pe.Next != nil {
				buffer.WriteString(fmt.Sprint(verticalMiddle))
			} else {
				buffer.WriteString(verticalEnd)
				buffer.WriteString("\n") // TOOD: ORS
			}
		}

		if e.Next() == nil {
			buffer.WriteString(horizontalStart)
			for pe := outrec.Head; pe != nil; pe = pe.Next {
				buffer.WriteString(horizontalBars[*pe.Key])
				if pe.Next != nil {
					buffer.WriteString(horizontalMiddle)
				} else {
					buffer.WriteString(horizontalEnd)
					buffer.WriteString("\n") // TOOD: ORS
				}
			}
		}

		os.Stdout.WriteString(buffer.String())
	}
}
