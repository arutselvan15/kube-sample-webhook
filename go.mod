module github.com/arutselvan15/estore-product-kube-webhook

require (
	github.com/arutselvan15/estore-common v1.0.3
	github.com/arutselvan15/estore-product-kube-client v1.0.3
	github.com/arutselvan15/go-utils v1.0.7
	github.com/google/uuid v1.1.1
	github.com/stretchr/testify v1.3.0
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/client-go v11.0.1-0.20190606204521-b8faab9c5193+incompatible
)

replace (
	k8s.io/api => k8s.io/api v0.0.0-20190918155943-95b840bb6a1f
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190913080033-27d36303b655
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190913080825-6f3bc4ba9215
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20190912054826-cd179ad6a269
	k8s.io/kubernetes => k8s.io/kubernetes v1.16.0
)
