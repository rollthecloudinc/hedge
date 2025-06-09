package utils

import (
	"bytes"
	"fmt"
	"errors"
	"os"
)

// EncodeStringToFixedBytes encodes a string into a fixed number of bytes.
func EncodeStringToFixedBytes(input string, fixedSize int) ([]byte, error) {
	if fixedSize <= 0 {
		return nil, errors.New("fixed size must be greater than zero")
	}

	encoded := []byte(input)

	// If the string is longer than the fixed size, truncate it.
	if len(encoded) > fixedSize {
		return encoded[:fixedSize], nil
	}

	// If the string is shorter, pad it with zero bytes.
	padding := make([]byte, fixedSize-len(encoded))
	return append(encoded, padding...), nil
}

// DecodeFixedBytesToString decodes a fixed-size byte array back into a string.
func DecodeFixedBytesToString(data []byte) string {
	return string(bytes.TrimRight(data, "\x00")) // Remove trailing zero bytes (padding)
}

// WriteFixedLengthStrings writes an array of strings to a binary file with fixed-length encoding.
func WriteFixedLengthStrings(filename string, strings []string, fixedSize int) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, s := range strings {
		encoded, err := EncodeStringToFixedBytes(s, fixedSize)
		if err != nil {
			return err
		}
		_, err = file.Write(encoded)
		if err != nil {
			return err
		}
	}
	return nil
}

// ReadFixedLengthString reads a single fixed-length string from a binary file at a given index.
func ReadFixedLengthString(filename string, index int, fixedSize int) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	offset := index * fixedSize
	data := make([]byte, fixedSize)

	// Seek to the correct position in the file
	_, err = file.Seek(int64(offset), 0)
	if err != nil {
		return "", err
	}

	// Read the fixed-length string
	_, err = file.Read(data)
	if err != nil {
		return "", err
	}

	// Decode and return the string
	return DecodeFixedBytesToString(data), nil
}

func main() {
	// Fixed size for each string (128 bytes)
	fixedSize := 128

	// Strings to store in the file
	strings := []string{"Hello", "World", "Golang", "This is a longer string!"}

	// File to store the fixed-length encoded strings
	filename := "fixed_length_strings_128.bin"

	// Write the strings to the file
	err := WriteFixedLengthStrings(filename, strings, fixedSize)
	if err != nil {
		fmt.Printf("Error writing strings to file: %v\n", err)
		return
	}
	fmt.Println("Strings written to file successfully.")

	// Read specific strings from the file
	for i := 0; i < len(strings); i++ {
		result, err := ReadFixedLengthString(filename, i, fixedSize)
		if err != nil {
			fmt.Printf("Error reading string at index %d: %v\n", i, err)
			continue
		}
		fmt.Printf("String at index %d: %s\n", i, result)
	}

	// Random access: Read the 2nd string (index 1, zero-based)
	index := 1
	randomResult, err := ReadFixedLengthString(filename, index, fixedSize)
	if err != nil {
		fmt.Printf("Error reading string at index %d: %v\n", index, err)
		return
	}
	fmt.Printf("Random access result (index %d): %s\n", index, randomResult)
}