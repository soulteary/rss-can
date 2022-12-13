module github.com/soulteary/RSS-Can

go 1.19

require (
	github.com/PuerkitoBio/goquery v1.8.0
	github.com/gorilla/feeds v1.1.1
	golang.org/x/net v0.4.0
	golang.org/x/text v0.5.0
	rogchap.com/v8go v0.7.0
)

require github.com/andybalholm/cascadia v1.3.1 // indirect

replace github.com/PuerkitoBio/goquery => ./pkg/PuerkitoBio/goquery

replace github.com/andybalholm/cascadia => ./pkg/andybalholm/cascadia

replace github.com/gorilla/feeds => ./pkg/gorilla/feeds
