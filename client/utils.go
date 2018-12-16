package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"
)

// MonthsShort list of short month names
var MonthsShort = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// DashLine contains dashes to fill a line of text within a codeblock on Discord on most mobile devices with 5+ inches screen
const DashLine = "-----------------------------------------------\n"

// FillOrLimit fill string with specific filler
func FillOrLimit(s interface{}, filler string, max int) string {
	str := fmt.Sprint(s)
	strLen := len(str)
	if strLen > max {
		return str[0:max]
	}
	if filler == "" {
		filler = " "
	}
	fillerLen := len(filler)
	return str + strings.Repeat(filler, (max-strLen)/fillerLen)
}

// WeiHexStrToFloat64 converts Wei Hex string to token balance
func WeiHexStrToFloat64(wei string) (balance float64, err error) {
	i := new(big.Float)
	i.SetString(wei)
	balance, err = strconv.ParseFloat(fmt.Sprint(i), 64)
	balance = balance / 1e18
	return
}

// ConvertNumber converts large numbers to into readable string
// Params:
// num float64   :
// dp int : number of decimal places to be rounded to. No rounding if 0 > precision. Max 18 DP.
func ConvertNumber(num float64, decimalPlaces int) string {
	divideBy := float64(1)
	name := ""
	if num >= 1e12 { // trillion
		divideBy = 1e12
		name = "Trillion"
	} else if num >= 1e9 { // billion
		divideBy = 1e9
		name = "Billion"
	} else if num >= 1e6 { // million
		divideBy = 1e6
		name = "Million"
	} else if num >= 1e3 { // thousand
		divideBy = 1e3
		name = "Thousand"
	}
	if decimalPlaces < 0 {
		// No decimal places
		decimalPlaces = 0
	} else if decimalPlaces > 18 {
		// Max decimal places
		decimalPlaces = 18
	}

	return fmt.Sprintf("%."+fmt.Sprint(decimalPlaces)+"f %s", num/divideBy, name)
}

// FormatTimeReverse formats time to string in the following format: HH:MM:SS DD-Mon
func FormatTimeReverse(t time.Time) string {
	return fmt.Sprintf("%02d:%02d:%02d %02d-%s", t.Hour(), t.Minute(), t.Second(), t.Day(), MonthsShort[t.Month()-1])
}

// FormatTS formats time to string in the following format: YYYY-MM-DD hh:mm:ss
func FormatTS(t time.Time) string {
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
}

// NowTS returns the current timestamp as a string
func NowTS() string {
	return FormatTS(time.Now().UTC())
}

// ReadFile reads and returns text content of the specified file
func ReadFile(pathToFile string) (text string, err error) {
	//open file for reading
	file, err := os.Open(pathToFile)
	defer file.Close()
	if err != nil {
		return
	}
	//initiate line by line scanner
	scanner := bufio.NewScanner(file)
	// Read each line
	for scanner.Scan() {
		text += scanner.Text()
	}
	err = scanner.Err()
	return
}

// WriteFile Writes file to system
func WriteFile(destinationPath string, text string, permission os.FileMode) error {
	return ioutil.WriteFile(destinationPath, []byte(text), permission)
}

// SaveJSONFile writes supplied data as foratted JSON
func SaveJSONFile(filename string, data interface{}) (err error) {
	dataBytes, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return
	}
	return ioutil.WriteFile(filename, dataBytes, 0644)
}

// SaveJSONFileLarge writes supplied data as foratted JSON
func SaveJSONFileLarge(filename string, data interface{}) (err error) {
	dataBytes, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return
	}
	file, err := os.Create(filename)
	if err != nil {
		fmt.Println("create failed")
		return
	}
	defer file.Close()
	_, err = file.Write(dataBytes)
	return
}
