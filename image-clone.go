package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

var repository string = "quay.io/mbtamuli"

func RegistryLogin(registry, username, password string) error {
	if username == "" && password == "" {
		return fmt.Errorf("username and password required")
	}
	cf, err := config.Load(os.Getenv("DOCKER_CONFIG"))
	if err != nil {
		return err
	}
	creds := cf.GetCredentialsStore(registry)
	if registry == name.DefaultRegistry {
		registry = authn.DefaultAuthKey
	}

	if err := creds.Store(types.AuthConfig{
		ServerAddress: registry,
		Username:      username,
		Password:      password,
	}); err != nil {
		return err
	}

	if err := cf.Save(); err != nil {
		return err
	}
	log.Printf("logged in via %s", cf.Filename)

	return nil
}

func ImageBackup(src string) error {
	if err := imageTag(src); err != nil {
		return err
	}
	return nil
}

func imageTag(src string) error {
	ref, err := name.ParseReference(src)
	if err != nil {
		return err
	}

	tag, err := name.NewTag(src)
	if err != nil {
		return err
	}

	img, err := remote.Image(ref)
	if err != nil {
		return err
	}

	nameWithoutRegistry := strings.ReplaceAll(ref.Context().Name(), ref.Context().RegistryStr(), "")
	nameWithoutNestedRepository := strings.ReplaceAll(nameWithoutRegistry, "/", "-")
	nameWithBackupRegistry := repository + "/" + nameWithoutNestedRepository[1:] + ":" + tag.TagStr()

	newRef, err := name.ParseReference(nameWithBackupRegistry)
	if err != nil {
		return err
	}
	return remote.Write(newRef, img, remote.WithAuthFromKeychain(authn.DefaultKeychain))
}
