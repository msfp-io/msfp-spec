package main

import (
	"errors"
	"strings"
)

// The language codes a media server gives, as MSFP sends them
// (spec/v0/catalog.md section 3, language tags): an ISO 639-2 code, /B
// (fre) or /T (fra), of a language ISO 639-1 names is sent as its 639-1
// code (fr), the rest of the tag kept; every tag in BCP 47's case
// conventions (spec/v0/core.md section 2.5); und, mul, zxx and mis not
// sent.

// iso639 maps each ISO 639-2 code, /B and /T, of a language that has an
// ISO 639-1 code to that code (from the iso-codes tables).
var iso639 = map[string]string{
	"aar": "aa", "abk": "ab", "afr": "af", "aka": "ak", "alb": "sq", "amh": "am", "ara": "ar",
	"arg": "an", "arm": "hy", "asm": "as", "ava": "av", "ave": "ae", "aym": "ay", "aze": "az",
	"bak": "ba", "bam": "bm", "baq": "eu", "bel": "be", "ben": "bn", "bih": "bh", "bis": "bi",
	"bod": "bo", "bos": "bs", "bre": "br", "bul": "bg", "bur": "my", "cat": "ca", "ces": "cs",
	"cha": "ch", "che": "ce", "chi": "zh", "chu": "cu", "chv": "cv", "cor": "kw", "cos": "co",
	"cre": "cr", "cym": "cy", "cze": "cs", "dan": "da", "deu": "de", "div": "dv", "dut": "nl",
	"dzo": "dz", "ell": "el", "eng": "en", "epo": "eo", "est": "et", "eus": "eu", "ewe": "ee",
	"fao": "fo", "fas": "fa", "fij": "fj", "fin": "fi", "fra": "fr", "fre": "fr", "fry": "fy",
	"ful": "ff", "geo": "ka", "ger": "de", "gla": "gd", "gle": "ga", "glg": "gl", "glv": "gv",
	"gre": "el", "grn": "gn", "guj": "gu", "hat": "ht", "hau": "ha", "heb": "he", "her": "hz",
	"hin": "hi", "hmo": "ho", "hrv": "hr", "hun": "hu", "hye": "hy", "ibo": "ig", "ice": "is",
	"ido": "io", "iii": "ii", "iku": "iu", "ile": "ie", "ina": "ia", "ind": "id", "ipk": "ik",
	"isl": "is", "ita": "it", "jav": "jv", "jpn": "ja", "kal": "kl", "kan": "kn", "kas": "ks",
	"kat": "ka", "kau": "kr", "kaz": "kk", "khm": "km", "kik": "ki", "kin": "rw", "kir": "ky",
	"kom": "kv", "kon": "kg", "kor": "ko", "kua": "kj", "kur": "ku", "lao": "lo", "lat": "la",
	"lav": "lv", "lim": "li", "lin": "ln", "lit": "lt", "ltz": "lb", "lub": "lu", "lug": "lg",
	"mac": "mk", "mah": "mh", "mal": "ml", "mao": "mi", "mar": "mr", "may": "ms", "mkd": "mk",
	"mlg": "mg", "mlt": "mt", "mon": "mn", "mri": "mi", "msa": "ms", "mya": "my", "nau": "na",
	"nav": "nv", "nbl": "nr", "nde": "nd", "ndo": "ng", "nep": "ne", "nld": "nl", "nno": "nn",
	"nob": "nb", "nor": "no", "nya": "ny", "oci": "oc", "oji": "oj", "ori": "or", "orm": "om",
	"oss": "os", "pan": "pa", "per": "fa", "pli": "pi", "pol": "pl", "por": "pt", "pus": "ps",
	"que": "qu", "roh": "rm", "ron": "ro", "rum": "ro", "run": "rn", "rus": "ru", "sag": "sg",
	"san": "sa", "sin": "si", "slk": "sk", "slo": "sk", "slv": "sl", "sme": "se", "smo": "sm",
	"sna": "sn", "snd": "sd", "som": "so", "sot": "st", "spa": "es", "sqi": "sq", "srd": "sc",
	"srp": "sr", "ssw": "ss", "sun": "su", "swa": "sw", "swe": "sv", "tah": "ty", "tam": "ta",
	"tat": "tt", "tel": "te", "tgk": "tg", "tgl": "tl", "tha": "th", "tib": "bo", "tir": "ti",
	"ton": "to", "tsn": "tn", "tso": "ts", "tuk": "tk", "tur": "tr", "twi": "tw", "uig": "ug",
	"ukr": "uk", "urd": "ur", "uzb": "uz", "ven": "ve", "vie": "vi", "vol": "vo", "wel": "cy",
	"wln": "wa", "wol": "wo", "xho": "xh", "yid": "yi", "yor": "yo", "zha": "za", "zho": "zh",
	"zul": "zu",
}

// sentTag is the tag a server sends for code, a media server's language
// code; an error for one that isn't sent.
func sentTag(code string) (string, error) {
	if len(code) < 2 || len(code) > 12 || strings.Trim(code, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-") != "" {
		return "", errors.New("not a tag: 2 to 12 characters of A-Z a-z 0-9 -")
	}
	subtags := strings.Split(code, "-")
	for i, s := range subtags {
		if s == "" {
			return "", errors.New("an empty subtag")
		}
		s = strings.ToLower(s)
		switch {
		case i == 0:
			if len(s) > 8 || !letters(s) {
				return "", errors.New("the language isn't 2 to 8 letters")
			}
			if short, ok := iso639[s]; ok {
				s = short
			}
			if s == "und" || s == "mul" || s == "zxx" || s == "mis" {
				return "", errors.New(s + " is not sent")
			}
		case len(s) == 2 && letters(s), len(s) == 3 && digits(s):
			s = strings.ToUpper(s)
		case len(s) == 4 && letters(s):
			s = strings.ToUpper(s[:1]) + s[1:]
		}
		subtags[i] = s
	}
	return strings.Join(subtags, "-"), nil
}

// validTag: a valid tag (catalog section 3, language tags) is one in the
// form a server sends: one that sentTag leaves as it is. Every part checks
// a tag it receives this way.
func validTag(s string) bool {
	t, err := sentTag(s)
	return err == nil && t == s
}

// languageCase is a media server's code (text) and the tag sent
// (output), or none (valid false).
type languageCase struct {
	Text   string `json:"text"`
	Valid  bool   `json:"valid"`
	Output string `json:"output,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// languageCases are media servers' language codes and what is sent.
func languageCases() []languageCase {
	var out []languageCase
	for _, c := range []string{"fr", "fre", "fra", "FRE", "ger", "deu", "chi", "zho", "cze", "ces", "wel", "dut", "gre",
		"rum", "per", "may", "tib", "alb", "arm", "baq", "geo", "ice", "mac", "mao", "bur", "slo", "fre-CA", "fre-ca",
		"en-us", "EN-GB", "chi-Hant-TW", "zh-hant-tw", "es-419", "yue", "fil", "tlh", "und", "mul", "zxx", "mis",
		"UND", "e", "fr_CA", "fr--CA", "fr-", "123", "abcdefghi", "français"} {
		tag, err := sentTag(c)
		ok, r := reason(err)
		out = append(out, languageCase{Text: c, Valid: ok, Output: tag, Reason: r})
	}
	return out
}

func letters(s string) bool { return strings.Trim(s, "abcdefghijklmnopqrstuvwxyz") == "" }
func digits(s string) bool  { return strings.Trim(s, "0123456789") == "" }
