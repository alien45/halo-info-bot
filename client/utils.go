package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"
)

// MonthsShort list of short month names
var MonthsShort = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// DashLine contains dashes to fill a line of text within a codeblock on Discord on most mobile devices with 5+ inches screen
const DashLine = "--------------------------------------------\n"

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

// FormatNum returns number formatted with commas
func FormatNum(num float64, dp int) (s string) {
	if math.IsInf(num, 0) || math.IsNaN(num) {
		num = 0
	}
	ar := strings.Split(fmt.Sprintf("%."+fmt.Sprint(dp)+"f", math.Abs(num)), ".")
	numDigits := len(ar[0])
	s = ar[0]
	for i := 1; i <= int((numDigits-1)/3); i++ {
		pos := numDigits - i*3
		s = s[:pos] + "," + s[pos:]
	}
	if dp > 0 && len(ar) > 1 {
		s += "." + ar[1]
	}
	if num < 0 {
		s = "-" + s
	}
	return
}

// FormatNumShort converts numbers to into readable string with initials of large number names such as B for Billion etc
//
// Params:
//
// @num float64 : number to convert. For integers cast to float64 first: float64(num)
//
// @dp  int	   : number of decimal places to be rounded to. No rounding if 0 > precision. Max 18 DP.
func FormatNumShort(num float64, dp int) string {
	if dp < 0 {
		dp = 0
	}
	e, n := 0, ""
	switch {
	case num < 1e3:
		break
	case num < 1e6:
		e, n = 3, "K"
		break
	case num < 1e9:
		e, n = 6, "M"
		break
	case num < 1e12:
		e, n = 9, "B"
		break
	case num < 1e15:
		e, n = 12, "T"
		break
	case num >= 1e15:
		e, n = 15, "Q"
		break
	}
	return fmt.Sprintf("%."+fmt.Sprint(dp)+"f %s", num/math.Pow10(e), n)
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

// SaveJSONFileLarge writes supplied data as foratted JSON. Create file if not exists.
func SaveJSONFileLarge(filename string, data interface{}) (err error) {
	dataBytes, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return
	}
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		fmt.Println("create failed")
		return
	}
	defer file.Close()
	_, err = file.Write(dataBytes)
	return
}

// AppendToFile append text to file
func AppendToFile(filepath, text string) (err error) {
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return
	}
	defer file.Close()
	_, err = file.WriteString(text)
	return
}

// AppendJSONToFile appends data as a single line json to file
func AppendJSONToFile(filepath, prefix string, data interface{}) (err error) {
	b, err := json.Marshal(data)
	if err != nil {
		return
	}
	return AppendToFile(filepath, prefix+string(b)+"\n")
}
