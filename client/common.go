package client

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

// MonthsShort list of short month names
var MonthsShort = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

const dashLine = "--------------------------------------------------\n"

// FillOrLimit fill string with specific filler
func FillOrLimit(str, filler string, max int) string {
	strLen := len(str)
	if strLen > max {
		return str[0:max]
	}
	fillerLen := len(filler)
	return str + strings.Repeat(filler, (max-strLen)/fillerLen)
}

// WeiHexStrToBalance converts Wei Hex string to token balance
func WeiHexStrToBalance(wei string) (balance float64, err error) {
	i := new(big.Float)
	i.SetString(wei)
	balance, err = strconv.ParseFloat(fmt.Sprint(i), 64)
	balance = balance / 1e18
	return
}

// ConvertNumber converts large numbers to into readable string
// Params:
// num float64   :
// precision int : number of decimal places to be rounded to. No rounding if 0 > precision > 18.
func ConvertNumber(num float64, precision int) string {
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
	if precision < 0 || precision > 18 {
		return fmt.Sprintf("%f %s", num/divideBy, name)
	}
	return fmt.Sprintf("%."+fmt.Sprint(precision)+"f %s", num/divideBy, name)
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
