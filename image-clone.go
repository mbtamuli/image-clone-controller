package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func RegistryLogin(registry, username, password string) error {
	reg, err := name.NewRegistry(registry)
	if err != nil {
		return err
	}
	serverAddress := reg.Name()
	if username == "" && password == "" {
		return fmt.Errorf("username and password required")
	}
	cf, err := config.Load(os.Getenv("DOCKER_CONFIG"))
	if err != nil {
		return fmt.Errorf("unable to load config: %s", err)
	}
	creds := cf.GetCredentialsStore(serverAddress)
	if serverAddress == name.DefaultRegistry {
		serverAddress = authn.DefaultAuthKey
	}

	if err := creds.Store(types.AuthConfig{
		ServerAddress: serverAddress,
		Username:      username,
		Password:      password,
	}); err != nil {
		return fmt.Errorf("unable to store credentials: %s", err)
	}

	if err := cf.Save(); err != nil {
		return fmt.Errorf("unable to save config: %s", err)
	}

	fmt.Printf("logged in via %s\n", cf.Filename)
	return nil
}

func ImageBackup(registry, repository, src string) (string, error) {
	ref, err := name.ParseReference(src)
	if err != nil {
		return "", fmt.Errorf("unable to parse source ref: %s", err)
	}

	tag, err := name.NewTag(src)
	if err != nil {
		return "", fmt.Errorf("unable to parse tag: %s", err)
	}

	desc, err := remote.Get(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return "", fmt.Errorf("unable to access remote image: %s", err)
	}

	img, err := desc.Image()
	if err != nil {
		return "", fmt.Errorf("failed to get image: %s", err)
	}

	newName := rename(ref, tag, registry, repository)
	newRef, err := name.ParseReference(newName)
	if err != nil {
		return "", fmt.Errorf("unable to parse new ref: %s", err)
	}

	return newRef.Name(), remote.Write(newRef, img, remote.WithAuthFromKeychain(authn.DefaultKeychain))
}

func rename(source name.Reference, tag name.Tag, registry, repository string) string {
	var destination string
	nameWithoutRegistry := strings.ReplaceAll(source.Context().Name(), source.Context().RegistryStr(), "")
	nameWithoutNestedRepository := strings.ReplaceAll(nameWithoutRegistry, "/", "-")
	destination = fmt.Sprintf("%s/%s/%s:%s", registry, repository, nameWithoutNestedRepository[1:], tag.TagStr())
	if strings.Contains(registry, "index.docker.io/v1") || registry == "" {
		destination = repository + "/" + nameWithoutNestedRepository[1:] + ":" + tag.TagStr()
	}
	return destination
}
