module github.com/soulteary/RSS-Can

go 1.19

require github.com/PuerkitoBio/goquery v1.8.0

require (
	github.com/andybalholm/cascadia v1.3.1 // indirect
	golang.org/x/net v0.2.0 // indirect
)

replace github.com/PuerkitoBio/goquery => ./pkg/PuerkitoBio/goquery

replace github.com/andybalholm/cascadia => ./pkg/andybalholm/cascadia
