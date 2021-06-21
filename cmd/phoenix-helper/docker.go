package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	docker "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/heroku/docker-registry-client/registry"
	"github.com/opencontainers/go-digest"
)

const (
	DockerRegistry = "eu.gcr.io/ae101-197818"

	noPush = false
)

func buildApplications(arch string) error {

	lg.Info("Building application for " + arch)

	outputDir := fmt.Sprintf("bin/%s", arch)

	if err := exec.Command("mkdir", "-p", outputDir).Run(); err != nil {
		return err
	}

	//Find applications
	cmdDir, err := os.Open("cmd")
	if err != nil {
		return err
	}

	applications, err := cmdDir.Readdir(0)
	if err != nil {
		return err
	}

	for _, application := range applications {

		cmd := exec.Command("go", "build", "-v", "-o", outputDir, "./cmd/"+application.Name())
		cmd.Env = append(os.Environ(),
			"GOOS=linux",
			"GOARCH="+arch,
		)

		stderr, err := cmd.StderrPipe()
		if err != nil {
			return err
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}

		go logReader(stderr, "build-application-stderr-"+arch)
		go logReader(stdout, "build-application-stdout-"+arch)
		lg.Info("Running command: " + cmd.String())
		if err := cmd.Start(); err != nil {
			return err
		}

		if err := cmd.Wait(); err != nil {
			return err
		}

	}

	return nil

}

func dockerBuildImages() error {
	currentDirectory, err := os.Getwd()
	if err != nil {
		return err
	}

	arches := []struct {
		Name string
		Host string
		Sha  string
	}{
		{
			Name: "amd64",
			Host: "unix:///var/run/docker.sock",
			Sha:  "sha256:2f3576726fd76cc8ac789e3500be4335e53c3c5800f0904b71513571b81f5b00",
		},
		{
			Name: "arm64",
			Host: "tcp://192.168.1.191:2375",
			Sha:  "sha256:5948f6fb4ca63fd9e4095229c301c57b2629e7f38fb4ac6506b5c0bacad76f5d",
		},
	}

	//Read auth key
	key, err := ioutil.ReadFile("docker_auth.json")
	if err != nil {
		return err
	}

	auth := types.AuthConfig{
		Username:      "_json_key",
		Password:      string(key),
		ServerAddress: "https://eu.gcr.io",
	}

	jsonAuth, err := json.Marshal(auth)
	if err != nil {
		return err
	}

	base64Auth := base64.URLEncoding.EncodeToString(jsonAuth)

	hub, err := registry.New("https://eu.gcr.io", auth.Username, auth.Password)
	if err != nil {
		return err
	}

	files, err := ioutil.ReadDir("./cmd")
	if err != nil {
		return err
	}

	for _, f := range files {

		application := f.Name()
		baseImage := fmt.Sprintf("%s/sandbox/%s", DockerRegistry, application)

		manifestList := &manifestlist.ManifestList{
			Versioned: manifest.Versioned{
				SchemaVersion: 1,
			},
			//Tag: "latest",
		}
		log.Printf("Manifest: %v\n", manifestList)

		for _, arch := range arches {
			image := fmt.Sprintf("%s:%s", baseImage, arch.Name)

			if err := buildApplications(arch.Name); err != nil {
				return err
			}

			cli, err := docker.NewClientWithOpts(
				docker.WithHost(arch.Host),
				docker.WithVersion("1.40"),
			)
			if err != nil {
				return err
			}
			defer cli.Close()

			filter := filters.NewArgs()
			filter.Add("reference", baseImage)
			//Clean up
			images, err := cli.ImageList(context.Background(), types.ImageListOptions{
				Filters: filter,
			})
			if err != nil {
				return err
			}

			for _, i := range images {

				removeResponse, err := cli.ImageRemove(context.Background(), i.ID, types.ImageRemoveOptions{
					Force:         true,
					PruneChildren: true,
				})
				if err != nil {
					lg.WithField("error", err).Info("Image remove return error")

				}
				for _, resp := range removeResponse {
					lg.Infof("Delete: %s, untagged: %s", resp.Deleted, resp.Untagged)
				}
			}

			/*
				containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{
					All: true,
				})
				if err != nil {
					panic(err)
				}

				for _, container := range containers {
					fmt.Printf("%s %s\n", container.ID[:10], container.Image)
				}*/

			reader, err := archive.TarWithOptions(currentDirectory, &archive.TarOptions{})

			lg.Infof("Building %s", image)
			buildResponse, err := cli.ImageBuild(context.Background(), reader, types.ImageBuildOptions{
				Tags: []string{image},
				BuildArgs: map[string]*string{
					"arg_application": &application,
					"arg_arch":        &arch.Name,
				},
				Dockerfile:     "Dockerfile",
				ForceRemove:    true,
				SuppressOutput: true,
			})
			if err != nil {
				return err
			}
			logReader(buildResponse.Body, "docker-build-"+application+"-arch"+arch.Name)

			if !noPush {
				lg.Infof("Pushing %s", image)
				pushResponse, err := cli.ImagePush(context.Background(), image, types.ImagePushOptions{
					All:          true,
					RegistryAuth: base64Auth,
				})
				if err != nil {
					return err
				}

				body, err := ioutil.ReadAll(pushResponse)
				if err != nil {
					return err
				}

				lines := strings.Split(string(body), "\n")

				//Fetch the id
				res := struct {
					Aux *struct {
						Tag    string
						Digest string
						Size   int64
					} `json:"aux"`
					Status string `json:"status"`
				}{}

				if err := json.Unmarshal([]byte(lines[len(lines)-2]), &res); err != nil {
					return err
				}

				lg.Infof("Got sha: '%s'\n", res.Aux.Digest)
				id, err := digest.Parse(res.Aux.Digest)
				if err != nil {
					return err

				}

				manifest := manifestlist.ManifestDescriptor{
					Descriptor: distribution.Descriptor{
						MediaType: "application/vnd.docker.distribution.manifest.v2+json",
						Digest:    id,
						Size:      res.Aux.Size,
					},
					Platform: manifestlist.PlatformSpec{
						Architecture: arch.Name,
						OS:           "linux",
					},
				}

				manifestList.Manifests = append(manifestList.Manifests, manifest)

				pushResponse.Close()

			}
		}

		if !noPush {

			dm, err := manifestlist.FromDescriptors(manifestList.Manifests)
			if err != nil {
				return err
			}

			_, payload, err := dm.Payload()
			if err != nil {
				return err
			}

			for i, _ := range dm.Manifests {
				m := &(dm.Manifests[i])
				m.Size = int64(len(payload))
			}

			_, payload, err = dm.Payload()
			if err != nil {
				return err
			}

			manifestUrl := fmt.Sprintf("ae101-197818/sandbox/%s", application)
			if err := hub.PutManifest(manifestUrl, "latest", dm); err != nil {
				return err
			}
		}

	}

	return nil

}

func logReader(reader io.ReadCloser, tag string) {

	buffer := make([]byte, 1024*50)
	for {
		n, err := reader.Read(buffer)
		if err == io.EOF || err == os.ErrClosed {
			break
		}
		if err != nil {
			break
		}

		if n > 0 {
			lg.WithField("tag", tag).Debug(string(buffer[0:n]))
		}

	}
}
