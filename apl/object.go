package apl

import (
	"fmt"
	"strings"
	"text/tabwriter"
)

// Object is a compound type that has keys and values.
//
// Values are accessed by indexing with keys.
//	Object[Key]
// Keys are usually strings, but dont have to be.
// To set a key, use indexed assignment:
//	Object[Name]←X
// This also works for vectors
//	Object[`k1`k2`k3] ← 5 6 7
//
// Keys are returned by #Object.
// Number of keys can also be obtained by ⍴Object.
//
// Indexing by vector returns a Dict with the specified keys.
//	Object["key1" "key2"].
//
// Method calls (calling a function stored in a key) or a go method
// for an xgo object cannot be applied directly:
//	Object[`f] R  ⍝ cannot be parsed
// Instead, assign it to a function variable, or commute:
//	f←Object[`f] ⋄ f R
//      Object[`f]⍨R
type Object interface {
	Value
	Keys() []Value
	At(Value) Value
	Set(Value, Value) error
}

// Dict is a dictionary object.
// A Dict is created with the L#R, where
// L is a key or a vector of keys and R conforming values.
// Dicts can be indexed with their keys.
// Example:
//	D←`alpha#1 2 3   ⍝ Single key
//	D←`a`b`c#1 2 3   ⍝ 3 Keys
//	D[`a]            ⍝ returns value 1
//	D[`a`c]          ⍝ returns a dict with 2 keys
type Dict struct {
	K []Value
	M map[Value]Value
}

func (d *Dict) Keys() []Value {
	return d.K
}

func (d *Dict) At(key Value) Value {
	if d.M == nil {
		return nil
	}
	return d.M[key]
}

// Set updates the value for the given key, or creates a new one,
// if the key does not exist.
// Keys must be valid variable names.
func (d *Dict) Set(key Value, v Value) error {
	if d.M == nil {
		d.M = make(map[Value]Value)
	}
	if _, ok := d.M[key]; ok == false {
		d.K = append(d.K, key.Copy())
	}
	d.M[key.Copy()] = v.Copy()
	return nil
}

func (d *Dict) String(f Format) string {
	if f.PP == -2 {
		return d.jsonString(f)
	} else if f.PP == -3 {
		return d.matString(f)
	}
	var buf strings.Builder
	tw := tabwriter.NewWriter(&buf, 1, 0, 1, ' ', 0)
	for _, k := range d.K {
		fmt.Fprintf(tw, "%s:\t%s\n", k.String(f), d.M[k].String(f))
	}
	tw.Flush()
	s := buf.String()
	if len(s) > 0 && s[len(s)-1] == '\n' {
		return s[:len(s)-1]
	}
	return s
}

func (d *Dict) Copy() Value {
	r := Dict{}
	if d.K != nil {
		r.K = make([]Value, len(d.K))
		for i := range d.K {
			r.K[i] = d.K[i].Copy()
		}
	}
	if d.M != nil {
		r.M = make(map[Value]Value)
		for k, v := range d.M {
			r.M[k.Copy()] = v.Copy()
		}
	}
	return &r
}

func (d *Dict) jsonString(f Format) string {
	var b strings.Builder
	b.WriteRune('{')
	keys := d.Keys()
	for i, key := range keys {
		if i > 0 {
			b.WriteRune(',')
		}
		k := key.String(f)
		val := d.At(key)
		v := val.String(f)
		b.WriteString(k)
		b.WriteRune(':')
		b.WriteString(v)
	}
	b.WriteRune('}')
	return b.String()
}

func (d *Dict) matString(f Format) string {
	var b strings.Builder
	b.WriteString("struct(")
	keys := d.Keys()
	for i, key := range keys {
		if i > 0 {
			b.WriteRune(',')
		}
		k := key.String(f)
		val := d.At(key)
		v := val.String(f)
		b.WriteString(k)
		b.WriteRune(',')
		b.WriteString(v)
	}
	b.WriteRune(')')
	return b.String()
}

func (a *Apl) ParseDict(prototype Value, s string) (*Dict, error) {
	if prototype != nil {
		_, ok := prototype.(*Dict)
		if ok == false {
			return nil, fmt.Errorf("ParseDict: prototype is not a dict: %T", prototype)
		}
	}
	return nil, fmt.Errorf("TODO ParseDict")
}
