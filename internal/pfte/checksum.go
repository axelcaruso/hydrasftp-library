// This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0.
// If a copy of the MPL was not distributed with this file, You can obtain one at
// https://mozilla.org/MPL/2.0/.

package pfte

import (
	"fmt"
	"hash/crc32"
	"io"
	"os"
)

// CalculateChecksum computes the CRC32 hash of a file.
// We use CRC32 because SHA256 is too slow for high-throughput transfer checks.
// We just want to know if the file got corrupted, not sign a contract.
func CalculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// IEEE is the standard used by Ethernet, Zip, etc. Fast and reliable.
	hasher := crc32.NewIEEE()

	// Copy the file content into the hasher in chunks (32KB buffer usually)
	// efficiently without loading the whole file into RAM.
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	// Return as hex string for easy comparison
	return fmt.Sprintf("%x", hasher.Sum32()), nil
}