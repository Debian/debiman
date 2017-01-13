package recode

import (
	"io"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// encodingForLang specifies which encoding should be used for a
// language if the encoding is not UTF-8. This mapping was taken from
// man-db-2.7.6.1/lib/encodings.c:directory_table
var encodingForLang = map[string]encoding.Encoding{
	"be":       charmap.Windows1251,
	"bg":       charmap.Windows1251,
	"cs":       charmap.ISO8859_2,
	"el":       charmap.ISO8859_7,
	"hr":       charmap.ISO8859_2,
	"hu":       charmap.ISO8859_2,
	"ja":       japanese.EUCJP,
	"ko":       korean.EUCKR,
	"lt":       charmap.ISO8859_13,
	"lv":       charmap.ISO8859_13,
	"mk":       charmap.ISO8859_5,
	"pl":       charmap.ISO8859_2,
	"ro":       charmap.ISO8859_2,
	"ru":       charmap.KOI8R,
	"sk":       charmap.ISO8859_2,
	"sl":       charmap.ISO8859_2,
	"sr@latin": charmap.ISO8859_2,
	"sr":       charmap.ISO8859_5,
	"tr":       charmap.ISO8859_9,
	"uk":       charmap.KOI8U,
	// TODO(later, all vi manpages in Debian are UTF-8):
	// "vi": TODO,
	"zh_CN": simplifiedchinese.GBK,
	"zh_SG": simplifiedchinese.GBK,
	// TODO(later, all zh_HK manpages in Debian are UTF-8):
	// is traditionalchinese.Big5 usable for BIG5HKSCS?
	// "zh_HK": TODO,
	"zh_TW": traditionalchinese.Big5,
}

// defaultEncoding is used for any language not contained in
// encodingForLang.
var defaultEncoding = charmap.ISO8859_1

func Reader(r io.Reader, lang string) io.Reader {
	enc, ok := encodingForLang[lang]
	if !ok {
		enc = defaultEncoding
	}
	return enc.NewDecoder().Reader(r)
}
