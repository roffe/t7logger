package symbol

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

type Symbol struct {
	Name string

	Number int

	Address uint32
	Length  uint16
	Mask    uint16
	Type    uint8

	Correctionfactor string
	Unit             string
}

func NewFromBytes(data []byte, symb_count int) *Symbol {
	var internall_address uint32
	for i := 0; i < 4; i++ {
		internall_address <<= 8
		internall_address |= uint32(data[i])
	}
	var symbol_length uint16
	if symb_count == 0 {
		symbol_length = 0x08
	} else {
		for i := 4; i <= 5; i++ {
			symbol_length <<= 8
			symbol_length |= uint16(data[i])
		}
	}

	var symbol_mask uint16
	for i := 6; i <= 7; i++ {
		symbol_mask <<= 8
		symbol_mask |= uint16(data[i])
	}

	symbol_type := data[8]

	return &Symbol{
		Name:    "Symbol-" + strconv.Itoa(symb_count),
		Number:  symb_count,
		Address: internall_address,
		Length:  symbol_length,
		Mask:    symbol_mask,
		Type:    symbol_type,
	}

}

func (s *Symbol) String() string {
	return fmt.Sprintf("%s #%d @%08X type: %02X len: %d", s.Name, s.Number, s.Address, s.Type, s.Length)
}

func LoadSymbols(filename string, cb func(string)) ([]*Symbol, error) {
	return ExtractFile(cb, filename, 0, "")
}

func ExtractFile(cb func(string), filename string, languageID int, m_current_softwareversion string) ([]*Symbol, error) {
	if filename == "" {
		return nil, errors.New("no filename given")
	}

	file, err := os.OpenFile(filename, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fstats, err := file.Stat()
	if err != nil {
		return nil, err
	}

	//	symbol_collection := make(map[string]Symbol)

	if !IsBinaryPackedVersion(file, 0x9B) {
		//log.Println("Not a binary packed version")
		cb("Not a binary packed symbol table")
		if err := nonBinaryPacked(cb, file, fstats); err != nil {
			return nil, err
		}
	} else {
		//log.Println("Binary packed version")
		cb("Found binary packed symbol table")
		return BinaryPacked(cb, file)

	}
	return nil, errors.New("not implemented")
}

func nonBinaryPacked(cb func(string), file *os.File, fstats fs.FileInfo) error {
	symbolListOffset, err := GetSymbolListOffSet(file, int(fstats.Size()))
	if err != nil {
		return err
	}
	log.Printf("Symbol list offset: %X", symbolListOffset)
	return nil
}

func BinaryPacked(cb func(string), file io.ReadSeeker) ([]*Symbol, error) {
	compr_created, addressTableOffset, compressedSymbolTable, err := extractCompressedSymbolTable(cb, file)
	if err != nil {
		return nil, err
	}

	//os.WriteFile("compressedSymbolNameTable.bin", compressedSymbolTable, 0644)
	if addressTableOffset == -1 {
		return nil, errors.New("could not find addressTableOffset table")
	}

	//ff, err := os.Create("compressedSymbolTable.bin")
	//if err != nil {
	//	return nil, err
	//}
	//defer ff.Close()
	file.Seek(int64(addressTableOffset), io.SeekStart)

	var (
		symb_count int
		symbols    []*Symbol
	)

	for {
		buff := make([]byte, 10)
		n, err := file.Read(buff)
		if err != nil {
			return nil, err
		}
		if n != 10 {
			return nil, errors.New("binaryPacked: not enough bytes read")
		}
		//ff.Write(buff)
		if int32(buff[0]) != 0x53 && int32(buff[1]) != 0x43 { // SC
			symbols = append(symbols, NewFromBytes(buff, symb_count))
			symb_count++
		} else {
			if pos, err := file.Seek(0, io.SeekCurrent); err == nil {
				log.Printf("EOT: %X", pos)
			}
			break
		}

	}
	//log.Println("Symbols found: ", symb_count)
	cb(fmt.Sprintf("Loaded %d symbols from binary", symb_count))

	if compr_created {
		//log.Println("Decoding packed symbol table")
		//cb("Decoding packed symbol table")
		symbolNames, err := ExpandCompressedSymbolNames(compressedSymbolTable)
		if err != nil {
			return nil, err
		}
		for i := 0; i < len(symbolNames)-1; i++ {
			symbols[i].Name = strings.TrimSpace(symbolNames[i])
			symbols[i].Unit = GetUnit(symbols[i].Name)
			symbols[i].Correctionfactor = GetCorrectionfactor(symbols[i].Name)
		}
	}

	/*
		ff2, err := os.OpenFile("symbols.txt", os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		defer ff2.Close()

		for _, s := range symbols {
			ff2.WriteString(fmt.Sprintf("%s\n", s.String()))
		}
	*/

	return symbols, nil

}

var searchPattern = []byte{
	0x00, 0x00, 0x04, 0x00,
	0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00,
	0x20, 0x00,
}

//var readNo int

func ExpandCompressedSymbolNames(in []byte) ([]string, error) {
	//if err := os.WriteFile("compressed-"+strconv.Itoa(readNo)+".bin", in, 0644); err != nil {
	//	log.Println(err)
	//}
	//readNo++
	var expandedFileSize int
	for i := 0; i < 4; i++ {
		expandedFileSize |= int(in[i]) << uint(i*8)
	}
	out := make([]byte, expandedFileSize)

	dll, err := syscall.LoadDLL("lzhuf.dll")
	if err != nil {
		log.Println(err)
		return nil, fmt.Errorf("error loading lzhuf.dll: %w", err)
	}
	defer dll.Release()

	decode, err := dll.FindProc("Decode")
	if err != nil {
		log.Println(err)
		return nil, fmt.Errorf("error finding Decode in lzhuf.dll: %w", err)
	}

	r0, r1, err := decode.Call(uintptr(unsafe.Pointer(&in[0])), uintptr(unsafe.Pointer(&out[0])))
	if r1 == 0 {
		if err != nil {
			return nil, fmt.Errorf("error decoding compressed symbol table: %w", err)
		}
	}

	//if err := os.WriteFile("uncompressed-"+strconv.Itoa(readNo)+".bin", out, 0644); err != nil {
	//	log.Println(err)
	//}

	if int(r0) != expandedFileSize {
		return nil, fmt.Errorf("decoded data size missmatch: %d != %d", r0, expandedFileSize)
	}

	return strings.Split(string(out), "\r\n"), nil
}

func extractCompressedSymbolTable(cb func(string), file io.ReadSeeker) (bool, int, []byte, error) {
	addressTableOffset := bytePatternSearch(file, searchPattern, 0x30000) - 0x06
	//log.Printf("Address table offset: %08X", addressTableOffset)
	cb(fmt.Sprintf("Address table offset: %08X", addressTableOffset))

	sramTableOffset := getAddressFromOffset(file, addressTableOffset-0x06)
	//log.Printf("SRAM table offset: %08X", sramTableOffset)
	cb(fmt.Sprintf("SRAM table offset: %08X", sramTableOffset))

	symbolTableOffset := getAddressFromOffset(file, addressTableOffset)
	//log.Printf("Symbol table offset: %08X", symbolTableOffset)
	cb(fmt.Sprintf("Symbol table offset: %08X", symbolTableOffset))

	symbolTableLength := getLengthFromOffset(file, addressTableOffset+0x04)
	//log.Printf("Symbol table length: %08X", symbolTableLength)
	cb(fmt.Sprintf("Symbol table length: %08X", symbolTableLength))

	if symbolTableLength > 0x1000 && symbolTableOffset > 0 && symbolTableOffset < 0x70000 {
		file.Seek(int64(symbolTableOffset), io.SeekStart)
		compressedSymbolTable := make([]byte, symbolTableLength)
		n, err := file.Read(compressedSymbolTable)
		if err != nil {
			return false, -1, nil, err
		}
		if n != symbolTableLength {
			return false, -1, nil, errors.New("did not read enough bytes for symbol table")
		}
		return true, addressTableOffset, compressedSymbolTable, nil
	}
	return false, -1, nil, errors.New("ecst: symbol table not found")
}

func getLengthFromOffset(file io.ReadSeeker, offset int) int {
	file.Seek(int64(offset), io.SeekStart)
	var val uint16
	if err := binary.Read(file, binary.BigEndian, &val); err != nil {
		panic(err)
	}
	return int(val)
}

func getAddressFromOffset(file io.ReadSeeker, offset int) int {
	file.Seek(int64(offset), io.SeekStart)
	var val uint32
	if err := binary.Read(file, binary.BigEndian, &val); err != nil {
		panic(err)
	}
	return int(val)
}

func bytePatternSearch(f io.ReadSeeker, search []byte, startOffset int64) int {
	f.Seek(startOffset, io.SeekStart)
	ix := 0
	r := bufio.NewReader(f)
	for ix < len(search) {
		b, err := r.ReadByte()
		if err != nil {
			return -1
		}
		if search[ix] == b {
			ix++
		} else {
			ix = 0
		}
		startOffset++
	}
	f.Seek(0, io.SeekStart) // Seeks to the beginning
	return int(startOffset - int64(len(search)))
}

func GetSymbolListOffSet(file *os.File, length int) (int, error) {
	retval := 0
	zerocount := 0
	var pos int64
	var err error

	for pos < int64(length) && retval == 0 {
		// Get current file position
		pos, err = file.Seek(0, io.SeekCurrent)
		if err != nil {
			return 0, err
		}
		b := make([]byte, 1)
		n, err := file.Read(b)
		if err != nil {
			return 0, err
		}
		if n != 1 {
			return 0, errors.New("read error")
		}
		if b[0] == 0x00 {
			zerocount++
		} else {
			if zerocount < 15 {
				zerocount = 0
			} else {
				retval = int(pos)
			}
		}
	}

	return -1, errors.New("Symbol list not found")
}

func ReadMarkerAddressContent(file *os.File, value byte, filelength int) (length, retval, val int, err error) {
	s, err := file.Stat()
	if err != nil {
		return
	}
	fileoffset := int(s.Size() - 0x90)

	file.Seek(int64(fileoffset), 0)
	inb := make([]byte, 0x90)
	n, err := file.Read(inb)
	if err != nil {
		return
	}
	if n != 0x90 {
		err = fmt.Errorf("ReadMarkerAddressContent: read %d bytes, expected %d", n, 0x90)
		return
	}
	for t := 0; t < 0x90; t++ {
		if inb[t] == value && inb[t+1] < 0x30 {
			// Marker found, read 6 bytes
			retval = fileoffset + t // 0x07FF70 + t
			length = int(inb[t+1])
			break
		}
	}

	file.Seek(int64(retval-length), 0)
	info := make([]byte, length)
	n, err = file.Read(info)
	if err != nil {
		return
	}
	if n != length {
		err = fmt.Errorf("ReadMarkerAddressContent: read %d bytes, expected %d", n, length)
		return
	}
	for bc := 0; bc < length; bc++ {
		val <<= 8
		val |= int(info[bc])
	}
	return
}

func IsBinaryPackedVersion(file *os.File, filelength int) bool {
	length, retval, val, err := ReadMarkerAddressContent(file, 0x9B, filelength)
	if err != nil {
		panic(err)
	}
	log.Printf("Length: %d, Retval: %X, Val: %X", length, retval, val)
	if retval > 0 && length < filelength && length > 0 {
		return true
	}
	return false
}
