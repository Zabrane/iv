package apl

import (
	"fmt"
	"io"
	"reflect"
	"strings"
	"text/tabwriter"
	"unicode"

	"github.com/ktye/iv/apl/scan"
)

// SetPP is called when a value is assigned to Quad-PP.
// If R is an integer, PP is set to this value.
// If R is a dict that maps from values to string, the format strings of the types are set.
// If R is the empty array, all format strings are removed and PP is reset.
//
// PP >= 0 sets the precision when printing floating point numbers.
// When nothing else is set, the default format string is %.(prec)G.
// PP < 0 always uses full precision for numbers and quoted format for strings (%q).
// Additionally, there are several special interpretations for PP < 0:
//  -1:  arrays formatted in a table
//  -2:  arrays formatted in a single line of json
//  -3:  arrays formatted in a single line compatible with matlab
//  -8:  integers formatted as octal numbers with 0 prefix
// -16:  integers formatted as hexadecimal numbers with 0x prefix, floats with %b (-123456p-78)
func (a *Apl) SetPP(R Value) error {
	if _, ok := R.(EmptyArray); ok {
		a.Format.PP = 0
		a.Format.Fmt = make(map[reflect.Type]string)
		return nil
	} else if d, ok := R.(Object); ok {
		keys := d.Keys()
		for _, k := range keys {
			v := d.At(k)
			if v != nil {
				if s, ok := v.(String); ok {
					a.Format.Fmt[reflect.TypeOf(k)] = string(s)
				}
			}
		}
	} else if n, ok := R.(Number); ok {
		if i, ok := n.ToIndex(); ok {
			a.Format.PP = i
			return nil
		}
	}
	return fmt.Errorf("illegal type for PP: %T", R)
}

// ArrayString can be used by an array implementation.
// It formats an n-dimensional array using a tabwriter for PP>=-1.
// Each dimension is terminated by k newlines, where k is the dimension index.
// For PP==-2, it uses a single line json notation with nested brackets and
// for PP==-3, it formats in a single line matlab syntax (rank <= 2).
func ArrayString(f Format, v Array) string {
	if f.PP == -2 {
		return jsonArray(f, v)
	} else if f.PP == -3 {
		return matArray(f, v)
	}
	shape := v.Shape()
	if len(shape) == 0 {
		return ""
	} else if len(shape) == 1 {
		s := make([]string, shape[0])
		for i := 0; i < shape[0]; i++ {
			s[i] = v.At(i).String(f)
		}
		return strings.Join(s, " ")
	}
	size := 1
	for _, n := range shape {
		size *= n
	}

	idx := make([]int, len(shape))
	inc := func() int {
		for i := 0; i < len(idx); i++ {
			k := len(idx) - 1 - i
			idx[k]++
			if idx[k] == shape[k] {
				idx[k] = 0
			} else {
				return i
			}
		}
		return -1 // should not happen
	}
	var buf strings.Builder
	tw := tabwriter.NewWriter(&buf, 1, 0, 1, ' ', tabwriter.AlignRight) // tabwriter.AlignRight)
	for i := 0; i < size; i++ {
		fmt.Fprintf(tw, "%s\t", v.At(i).String(f))
		if term := inc(); term > 0 {
			for k := 0; k < term; k++ {
				fmt.Fprintln(tw)
			}
		} else if term == -1 {
			fmt.Fprintln(tw)
		}
	}
	tw.Flush()
	s := buf.String()
	if len(s) > 0 && s[len(s)-1] == '\n' {
		// Don't print the final newline.
		return s[:len(s)-1]
	}
	return s
}

// stringArray converts the array to a string array of the same shape.
// All elements are printed with the current PP.
func stringArray(f Format, v Array) StringArray {
	sa := StringArray{Dims: CopyShape(v), Strings: make([]string, v.Size())}
	for i := range sa.Strings {
		sa.Strings[i] = v.At(i).String(f)
	}
	return sa
}

// jsonArray is used for PP=-2
func jsonArray(f Format, v Array) string {
	sa := stringArray(f, v)
	var vector func(v StringArray) string
	vector = func(S StringArray) string {
		if len(S.Dims) == 1 {
			return "[" + strings.Join(S.Strings[:S.Dims[0]], ",") + "]"
		}
		vec := make([]string, S.Dims[0])
		inc := Prod(S.Dims[1:])
		for i := 0; i < S.Dims[0]; i++ {
			sub := StringArray{Dims: S.Dims[1:], Strings: S.Strings[i*inc:]}
			vec[i] = vector(sub)
		}
		return "[" + strings.Join(vec, ",") + "]"
	}
	return vector(sa)
}

// matArray is used for PP=-3. It only supported for rank 1 and 2.
func matArray(f Format, v Array) string {
	sa := stringArray(f, v)
	if len(sa.Dims) == 1 {
		return "[" + strings.Join(sa.Strings, ",") + "]"
	} else if len(sa.Dims) != 2 {
		return "[rank error]"
	}
	var b strings.Builder
	b.WriteString("[")
	off := 0
	for i := 0; i < sa.Dims[0]; i++ {
		b.WriteString(strings.Join(sa.Strings[off:off+sa.Dims[1]], ","))
		if i < sa.Dims[0]-1 {
			b.WriteString(";")
		}
		off += sa.Dims[1]
	}
	b.WriteString("]")
	return b.String()
}

// ParseArray parses a rectangular n-dimensional array from a string representation.
// The result will have the same type as the prototype, or an error is returned.
// If the prototype is nil, a mixed array is returned.
// The function can parse arrays that have been formatted with ¯1⍕, ¯2⍕ and ¯3⍕.
// Json arrays (¯2⍕) can only be parsed, if they don't contain complex numbers.
func (a *Apl) ParseArray(prototype Value, s string) (Value, error) {
	v, err := a.ScanRankArray(strings.NewReader(s), -1)
	if err != nil {
		return nil, fmt.Errorf("parse array: %s", err)
	}

	if prototype != nil {
		if u, ok := prototype.(Uniform); ok {
			res, ok := a.Unify(v.(Array), true)
			if ok == false {
				return nil, fmt.Errorf("parse array: array has no uniform type")
			}
			if reflect.TypeOf(res) != reflect.TypeOf(u) {
				return nil, fmt.Errorf("parse array: result has wrong type %T != %T", res, u)
			}
			return res, nil
		}
	}
	return v, nil
}

// ScanRankArray returns the next sub-array from a RuneScanner of a given rank.
// If rank is 0, it returns a Value that is not an array.
// If the rank is negative, it is not restricted.
// If result may have a smaller rank than requested without an error.
// The format is the same as for ParseArray.
func (a *Apl) ScanRankArray(s io.RuneScanner, rank int) (Value, error) {
	var values []Value
	c := 0
	var shape []int
	for {
		r, _, err := s.ReadRune()
		if err == io.EOF {
			break
		} else if r == '\n' || r == ';' || r == ']' {
			if len(values) == 0 {
				continue
			}
			c++
			if c == rank {
				break
			} else if c > len(shape) {
				if shape == nil {
					shape = []int{len(values)}
				} else {
					p := Prod(shape)
					shape = append([]int{len(values) / p}, shape...)
				}
			}
		} else if unicode.IsSpace(r) || r == ',' || r == '[' || r == '(' || r == ')' {
			continue
		} else if r == '"' { // Parse a string.
			c = 0
			s.UnreadRune()
			if str, err := scan.ReadString(s); err != nil {
				return nil, fmt.Errorf("parse array: %s", err)
			} else {
				if rank == 0 {
					return String(str), nil
				}
				values = append(values, String(str))
			}

		} else { // Parse an number.
			c = 0
			s.UnreadRune()
			num, err := scan.ScanNumber(s)
			if err != nil {
				return nil, fmt.Errorf("parse array: %s", err)
			}
			if n, err := a.Tower.Parse(num); err != nil {
				return nil, fmt.Errorf("parse array: %s", err)
			} else {
				if rank == 0 {
					return n, nil
				}
				values = append(values, n.Number)
			}
		}
	}
	// The algorithm does not check if the array is uniform in between.
	// We just test at the end, if the size matches the shape. This may include false positives.
	if len(values) == 0 {
		return nil, io.EOF
	}
	if rank < 0 {
		// For rank < 0, we read everything. Data could be closed or not.
		rank = len(shape)
		if Prod(shape) == len(values) {
			rank = len(shape) - 1
		}
	}
	for i := 0; i <= rank-len(shape); i++ {
		p := Prod(shape)
		if len(shape) == 0 {
			p = 1
		} else if p == 0 {
			return nil, fmt.Errorf("parse array: divide by zero: values: %v shape: %v", values, shape)
		}
		shape = append([]int{len(values) / p}, shape...)
		if Prod(shape) != len(values) {
			return nil, fmt.Errorf("parse array: array is not rectangular: ×/%v ≠ %v", shape, len(values))
		}
		// Continue and fill leading 1s if the rank is higher than data.
	}
	return MixedArray{
		Dims:   shape,
		Values: values,
	}, nil
}
