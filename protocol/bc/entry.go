package bc

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"reflect"

	"github.com/golang/protobuf/proto"

	"github.com/bytom/crypto/sha3pool"
	"github.com/bytom/encoding/blockchain"
	"github.com/bytom/errors"
)

// Entry is the interface implemented by each addressable unit in a
// blockchain: transaction components such as spends, issuances,
// outputs, and retirements (among others), plus blockheaders.
type Entry interface {
	proto.Message

	// type produces a short human-readable string uniquely identifying
	// the type of this entry.
	typ() string

	// writeForHash writes the entry's body for hashing.
	writeForHash(w io.Writer)
}

var errInvalidValue = errors.New("invalid value")

// EntryID computes the identifier of an entry, as the hash of its
// body plus some metadata.
func EntryID(e Entry) (hash Hash) {
	if e == nil {
		return hash
	}

	// Nil pointer; not the same as nil interface above. (See
	// https://golang.org/doc/faq#nil_error.)
	if v := reflect.ValueOf(e); v.Kind() == reflect.Ptr && v.IsNil() {
		return hash
	}
	fmt.Println("entryID ....\n", e.String())

	hasher := sha3pool.Get256()
	defer sha3pool.Put256(hasher)

	hasher.Write([]byte("entryid:"))
	fmt.Println("entry id is:", e.typ())
	hasher.Write([]byte(e.typ()))
	hasher.Write([]byte{':'})

	bh := sha3pool.Get256()
	defer sha3pool.Put256(bh)

	e.writeForHash(bh)

	var innerHash [32]byte
	bh.Read(innerHash[:])

	hasher.Write(innerHash[:])

	fmt.Println("innerHash", hex.EncodeToString(innerHash[:]))

	hash.ReadFrom(hasher)

	fmt.Println("hash.String()", hash.String())

	return hash
}

var byte32zero [32]byte

// mustWriteForHash serializes the object c to the writer w, from which
// presumably a hash can be extracted.
//
// This function may panic with an error from the underlying writer,
// and may produce errors of its own if passed objects whose
// hash-serialization formats are not specified. It MUST NOT produce
// errors in other cases.
func mustWriteForHash(w io.Writer, c interface{}) {
	if err := writeForHash(w, c); err != nil {
		panic(err)
	}
}

func writeForHash(w io.Writer, c interface{}) error {
	fmt.Println("writeForHash:")

	switch v := c.(type) {
	case byte:
		fmt.Println("byte")
		fmt.Println(hex.EncodeToString([]byte{v}))
		_, err := w.Write([]byte{v})
		return errors.Wrap(err, "writing byte for hash")
	case uint64:
		fmt.Println("unint64")
		buf := [8]byte{}
		binary.LittleEndian.PutUint64(buf[:], v)
		fmt.Println(hex.EncodeToString(buf[:]))
		_, err := w.Write(buf[:])
		return errors.Wrapf(err, "writing uint64 (%d) for hash", v)
	case []byte:
		fmt.Println("[]byte")
		_, err := blockchain.WriteVarstr31(w, v)
		return errors.Wrapf(err, "writing []byte (len %d) for hash", len(v))
	case [][]byte:
		fmt.Println("[][]byte")
		_, err := blockchain.WriteVarstrList(w, v)

		return errors.Wrapf(err, "writing [][]byte (len %d) for hash", len(v))
	case string:
		fmt.Println("string")
		fmt.Println(v)
		_, err := blockchain.WriteVarstr31(w, []byte(v))
		return errors.Wrapf(err, "writing string (len %d) for hash", len(v))
	case *Hash:
		fmt.Println("*hash")
		if v == nil {
			_, err := w.Write(byte32zero[:])
			fmt.Println(hex.EncodeToString(byte32zero[:]))
			return errors.Wrap(err, "writing nil *Hash for hash")
		}
		_, err := w.Write(v.Bytes())
		fmt.Println(hex.EncodeToString(v.Bytes()))
		return errors.Wrap(err, "writing *Hash for hash")
	case *AssetID:
		fmt.Println("*assetID")
		if v == nil {
			_, err := w.Write(byte32zero[:])
			fmt.Println(hex.EncodeToString(byte32zero[:]))
			return errors.Wrap(err, "writing nil *AssetID for hash")
		}
		_, err := w.Write(v.Bytes())
		fmt.Println(hex.EncodeToString(v.Bytes()))
		return errors.Wrap(err, "writing *AssetID for hash")
	case Hash:
		fmt.Println("hash")
		fmt.Println(v)
		_, err := v.WriteTo(w)
		return errors.Wrap(err, "writing Hash for hash")
	case AssetID:
		fmt.Println("assetID")
		fmt.Println(v)
		_, err := v.WriteTo(w)
		return errors.Wrap(err, "writing AssetID for hash")
	}

	// The two container types in the spec (List and Struct)
	// correspond to slices and structs in Go. They can't be
	// handled with type assertions, so we must use reflect.
	switch v := reflect.ValueOf(c); v.Kind() {
	case reflect.Ptr:
		fmt.Println("reflect.Ptr")
		if v.IsNil() {
			return nil
		}
		elem := v.Elem()
		return writeForHash(w, elem.Interface())
	case reflect.Slice:
		fmt.Println("reflect.Slice")
		l := v.Len()
		if _, err := blockchain.WriteVarint31(w, uint64(l)); err != nil {
			return errors.Wrapf(err, "writing slice (len %d) for hash", l)
		}
		for i := 0; i < l; i++ {
			c := v.Index(i)
			if !c.CanInterface() {
				return errInvalidValue
			}
			if err := writeForHash(w, c.Interface()); err != nil {
				return errors.Wrapf(err, "writing slice element %d for hash", i)
			}
		}
		return nil

	case reflect.Struct:
		fmt.Println("reflect.Struct")
		typ := v.Type()
		for i := 0; i < typ.NumField(); i++ {
			c := v.Field(i)
			if !c.CanInterface() {
				return errInvalidValue
			}
			if err := writeForHash(w, c.Interface()); err != nil {
				t := v.Type()
				f := t.Field(i)
				return errors.Wrapf(err, "writing struct field %d (%s.%s) for hash", i, t.Name(), f.Name)
			}
		}
		return nil
	}

	return errors.Wrap(fmt.Errorf("bad type %T", c))
}
