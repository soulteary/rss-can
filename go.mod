module github.com/soulteary/RSS-Can

go 1.22

require (
	github.com/JohannesKaufmann/html-to-markdown v1.5.0
	github.com/PuerkitoBio/goquery v1.9.1
	github.com/gin-contrib/gzip v1.0.0
	github.com/gin-gonic/gin v1.9.1
	github.com/go-redis/redis/v8 v8.11.5
	github.com/go-rod/rod v0.114.8
	github.com/gorilla/feeds v1.1.2
	github.com/muesli/cache2go v0.0.0-20221011235721-518229cd8021
	go.uber.org/zap v1.27.0
	golang.org/x/net v0.24.0
	golang.org/x/text v0.14.0
	rogchap.com/v8go v0.9.0
)

require (
	github.com/andybalholm/cascadia v1.3.2 // indirect
	github.com/bytedance/sonic v1.11.3 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/chenzhuoyu/base64x v0.0.0-20230717121745-296ad89f973d // indirect
	github.com/chenzhuoyu/iasm v0.9.1 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/gabriel-vasile/mimetype v1.4.3 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.19.0 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/google/go-cmp v0.5.8 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.2.7 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/onsi/gomega v1.19.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.12 // indirect
	github.com/ysmood/fetchup v0.2.4 // indirect
	github.com/ysmood/goob v0.4.0 // indirect
	github.com/ysmood/got v0.39.4 // indirect
	github.com/ysmood/gson v0.7.3 // indirect
	github.com/ysmood/leakless v0.8.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/arch v0.7.0 // indirect
	golang.org/x/crypto v0.22.0 // indirect
	golang.org/x/sys v0.19.0 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect

)

replace github.com/PuerkitoBio/goquery => ./pkg/PuerkitoBio/goquery

replace github.com/andybalholm/cascadia => ./pkg/andybalholm/cascadia

replace github.com/gorilla/feeds => ./pkg/gorilla/feeds

replace github.com/JohannesKaufmann/html-to-markdown => ./pkg/JohannesKaufmann/html-to-markdown

replace github.com/muesli/cache2go => ./pkg/muesli/cache2go
