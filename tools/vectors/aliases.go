package main

import "msfp-media/tools/vectors/rules"

// MSFP Media's rules and helpers, from msfp-media's generator (the rules
// package), by the names this program's code uses. MSFP depends on MSFP
// Media, never the reverse.

type (
	obj                = rules.Object
	verdict            = rules.Verdict
	mediaTicket        = rules.MediaTicket
	mediaNameCase      = rules.NameCase
	playbackAnswer     = rules.Answer
	playbackAnswerCase = rules.AnswerCase
	playbackAsked      = rules.Asked
	playbackCopy       = rules.Copy
	playbackLimits     = rules.Limits
	playbackLive       = rules.Live
	playbackStarted    = rules.Started
	playbackState      = rules.State
	playbackUse        = rules.Use
)

const (
	maxInt             = rules.MaxInt
	pbNow              = rules.Now
	pbPID              = rules.PID
	pbRoot             = rules.Root
	pbFilmKey          = rules.FilmKey
	pbTrackKeyA        = rules.TrackKeyA
	playbackMaxStreams = rules.MaxStreams
)

var (
	reason              = rules.Reason
	str                 = rules.Str
	num                 = rules.Num
	sameReason          = rules.SameReason
	encode              = rules.Encode
	mediaType           = rules.MediaType
	cleanName           = rules.CleanName
	checkIJSON          = rules.CheckIJSON
	checkDecimal        = rules.CheckDecimal
	checkInteger        = rules.CheckInteger
	checkMediaName      = rules.CheckMediaName
	checkRange          = rules.CheckRange
	groupName           = rules.GroupName
	looksLikeGroup      = rules.LooksLikeGroup
	paragraphs          = rules.Paragraphs
	tail                = rules.Tail
	sentenceEnd         = rules.SentenceEnd
	pbObj               = rules.Obj
	playbackDecode      = rules.Decode
	playbackInt         = rules.Int
	playbackEncode      = rules.Compact
	playbackStatus      = rules.Status
	playbackTitleKind   = rules.TitleKind
	playbackPickCopy    = rules.PickCopy
	playbackPickAudio   = rules.PickAudio
	playbackVideoWithin = rules.VideoWithin
	playbackAudioWithin = rules.AudioWithin
	playbackEtagForm    = rules.EtagForm
	playbackObjIDForm   = rules.ObjIDForm
	playbackBuiltName   = rules.BuiltName
	playbackPurposeOf   = rules.PurposeOf
	playbackKeepAnswer  = rules.KeepAnswer
)
