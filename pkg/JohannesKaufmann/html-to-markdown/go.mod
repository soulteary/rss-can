module github.com/JohannesKaufmann/html-to-markdown

go 1.22

require (
	github.com/PuerkitoBio/goquery v1.9.1
	github.com/sebdah/goldie/v2 v2.5.3
	github.com/yuin/goldmark v1.4.14
	golang.org/x/net v0.22.0
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/PuerkitoBio/goquery => ../../PuerkitoBio/goquery

require (
	github.com/andybalholm/cascadia v1.3.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sergi/go-diff v1.2.0 // indirect
)
