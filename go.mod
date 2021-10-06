module github.com/jfrog/froggit-go

go 1.16

require (
	github.com/gfleury/go-bitbucket-v1 v0.0.0-20210826163055-dff2223adeac
	github.com/google/go-github/v38 v38.1.0
	github.com/google/uuid v1.3.0
	github.com/ktrysmt/go-bitbucket v0.9.24
	github.com/stretchr/testify v1.7.0
	github.com/xanzy/go-gitlab v0.50.3
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5 // indirect
	golang.org/x/oauth2 v0.0.0-20210819190943-2bc19b11175f
)

exclude (
	golang.org/x/crypto v0.0.0-20190308221718-c2843e01d9a2
	golang.org/x/text v0.3.2
	golang.org/x/text v0.3.3
)
