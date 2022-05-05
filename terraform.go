package sls

import (
	"fmt"
	"path/filepath"
)

// Terraform stands for Terraform it holds the path to terraform
// binary as well as the Repo for global resources to be provisioned into
// the developer local aws account. Dir is where we will clone our
// global resource's repo.
type Terraform struct {
	BinaryDir       string
	BinaryName      string
	Version         string
	GlobalResources GlobalResources
}

type TFBackend struct {
	Bucket      string
	Key         string
	Region      Region
	DynamoTable string
}

func NewTFBackend(prefix Prefix, label string) TFBackend {
	if label == RemoteStateTF {
		return TFBackend{}
	}

	return TFBackend{
		Bucket:      fmt.Sprintf("%s-tf-%s", prefix, RemoteStateTF),
		Key:         fmt.Sprintf("%s/%s.tfstate", label, label),
		Region:      prefix.Region,
		DynamoTable: fmt.Sprintf("%s-tf-%s-lock", prefix, RemoteStateTF),
	}
}

type TFResource struct {
	Dir       string
	StateFile string
	PlanFile  string
	Name      string
	Backend   TFBackend
	Vars      map[string]string
}

func NewTFResource(rootDir string, prefix Prefix, label string) TFResource {
	vars := map[string]string{
		"env": prefix.Env(),
	}

	return TFResource{
		Dir:       rootDir,
		StateFile: "",
		PlanFile:  filepath.Join(rootDir, fmt.Sprintf("%s.plan.out", label)),
		Name:      label,
		Backend:   NewTFBackend(prefix, label),
		Vars:      vars,
	}
}

func (tr TFResource) IsBackend() bool {
	return tr.Name != RemoteStateTF
}

type GlobalResources struct {
	RemoteState TFResource
	Repo        Repo
	RootDir     string
	Config      map[string]TFResource
}

func (gr GlobalResources) LambdaDeployBucket() (TFResource, bool) {
	r, ok := gr.Config[LambdaDeployTF]
	return r, ok
}

func (gr GlobalResources) KeyPair() (TFResource, bool) {
	r, ok := gr.Config[KeyPairTF]
	return r, ok
}

func (gr GlobalResources) Messaging() (TFResource, bool) {
	r, ok := gr.Config[MessagingTF]
	return r, ok
}

func (gr GlobalResources) Cognito() (TFResource, bool) {
	r, ok := gr.Config[CognitoTF]
	return r, ok
}

func (gr GlobalResources) Networking() (TFResource, bool) {
	r, ok := gr.Config[NetworkingTF]
	return r, ok
}
