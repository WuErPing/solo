package plantuml

import "regexp"

// darkSkinparams re-colours the common PlantUML element families for a dark
// app theme. Values mirror the GitHub dark palette. They are injected before
// the user's own directives, so explicit skinparams/!theme in the diagram
// still win.
const darkSkinparams = `skinparam backgroundColor transparent
skinparam defaultFontColor #C9D1D9
skinparam shadowing false
skinparam ArrowColor #8B949E
skinparam ArrowFontColor #C9D1D9
skinparam ParticipantBorderColor #30363D
skinparam ParticipantBackgroundColor #161B22
skinparam ParticipantFontColor #C9D1D9
skinparam ActorBorderColor #30363D
skinparam ActorBackgroundColor #161B22
skinparam ActorFontColor #C9D1D9
skinparam classBorderColor #30363D
skinparam classBackgroundColor #161B22
skinparam classFontColor #C9D1D9
skinparam classArrowColor #8B949E
skinparam stateBorderColor #30363D
skinparam stateBackgroundColor #161B22
skinparam stateFontColor #C9D1D9
skinparam activityBorderColor #30363D
skinparam activityBackgroundColor #161B22
skinparam activityFontColor #C9D1D9
skinparam activityArrowColor #8B949E
skinparam activityDiamondBorderColor #30363D
skinparam componentBorderColor #30363D
skinparam componentBackgroundColor #161B22
skinparam componentFontColor #C9D1D9
skinparam sequenceBorderColor #30363D
skinparam sequenceGroupBackgroundColor #161B22
skinparam sequenceGroupBorderColor #30363D
skinparam sequenceGroupFontColor #C9D1D9
skinparam noteBorderColor #30363D
skinparam noteBackgroundColor #161B22
skinparam noteFontColor #C9D1D9
skinparam packageBorderColor #30363D
skinparam packageBackgroundColor #161B22
skinparam packageFontColor #C9D1D9
skinparam rectangleBorderColor #30363D
skinparam rectangleBackgroundColor #161B22
skinparam usecaseBorderColor #30363D
skinparam usecaseBackgroundColor #161B22
skinparam usecaseFontColor #C9D1D9
skinparam databaseBorderColor #30363D
skinparam databaseBackgroundColor #161B22
skinparam databaseFontColor #C9D1D9`

var startUmlLine = regexp.MustCompile(`(?mi)^[ \t]*@startuml[^\n]*$`)

// applyDarkSkinparams inserts the dark preamble after the first @startuml
// line when present, and prepends it otherwise (PlantUML accepts source
// without an explicit start marker, as does the compatible-server path).
func applyDarkSkinparams(source string) string {
	if match := startUmlLine.FindStringIndex(source); match != nil {
		return source[:match[1]] + "\n" + darkSkinparams + source[match[1]:]
	}
	return darkSkinparams + "\n" + source
}
