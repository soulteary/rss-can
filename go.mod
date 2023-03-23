module github.com/soulteary/RSS-Can

go 1.19

require (
	github.com/JohannesKaufmann/html-to-markdown v1.3.6
	github.com/PuerkitoBio/goquery v1.8.0
	github.com/gin-contrib/gzip v0.0.6
	github.com/gin-gonic/gin v1.8.2
	github.com/go-redis/redis/v8 v8.11.5
	github.com/go-rod/rod v0.112.2
	github.com/gorilla/feeds v1.1.1
	github.com/muesli/cache2go v0.0.0-00010101000000-000000000000
	go.uber.org/zap v1.24.0
	golang.org/x/net v0.4.0
	golang.org/x/text v0.8.0
	rogchap.com/v8go v0.7.0
)

require (
	github.com/andybalholm/cascadia v1.3.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-playground/locales v0.14.0 // indirect
	github.com/go-playground/universal-translator v0.18.0 // indirect
	github.com/go-playground/validator/v10 v10.11.1 // indirect
	github.com/goccy/go-json v0.9.11 // indirect
	github.com/google/go-cmp v0.5.8 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/leodido/go-urn v1.2.1 // indirect
	github.com/mattn/go-isatty v0.0.16 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/onsi/gomega v1.19.0 // indirect
	github.com/pelletier/go-toml/v2 v2.0.6 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/ugorji/go/codec v1.2.7 // indirect
	github.com/ysmood/goob v0.4.0 // indirect
	github.com/ysmood/gson v0.7.3 // indirect
	github.com/ysmood/leakless v0.8.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/goleak v1.1.12 // indirect
	go.uber.org/multierr v1.9.0 // indirect
	golang.org/x/crypto v0.4.0 // indirect
	golang.org/x/sys v0.5.0 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect

)

replace github.com/PuerkitoBio/goquery => ./pkg/PuerkitoBio/goquery

replace github.com/andybalholm/cascadia => ./pkg/andybalholm/cascadia

replace github.com/gorilla/feeds => ./pkg/gorilla/feeds

replace github.com/JohannesKaufmann/html-to-markdown => ./pkg/JohannesKaufmann/html-to-markdown

replace github.com/muesli/cache2go => ./pkg/muesli/cache2go
