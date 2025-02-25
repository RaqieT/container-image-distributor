package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Structs

type Repository struct {
	Name               string            `json:"name"`
	AdditionalNames    []string          `json:"additionalNames,omitempty"`
	Registry           string            `json:"registry"`
	Suffix             string            `json:"suffix,omitempty"`
	DestinationMapping map[string]string `json:"destinationMappings,omitempty"`
}

func (r Repository) GetRegistryPath() string {
	if len(r.Suffix) == 0 {
		return r.Registry
	}

	return r.Registry + "/" + r.Suffix
}

type Config struct {
	Repositories       []Repository      `json:"repositories"`
	DestinationMapping map[string]string `json:"destinationMappings,omitempty"`
}

// Errors

type RepositoryNotFoundForDestinationError struct {
	destination string
}

func (e RepositoryNotFoundForDestinationError) Error() string {
	return fmt.Sprintf("Could not find repository matching %s destination", e.destination)
}

type RepositoryNotFoundForSourceError struct {
	source string
}

func (e RepositoryNotFoundForSourceError) Error() string {
	return fmt.Sprintf("Could not find repository matching %s source image", e.source)
}

func main() {
	var containerTool string
	var sourceImage string
	var destination string
	var overrideTag string
	var force bool

	flag.StringVar(&containerTool, "container-tool", "docker", "podman/docker")
	flag.StringVar(&containerTool, "c", containerTool, "alias for -container-tool")

	flag.StringVar(&sourceImage, "image", "", "image that will be used")
	flag.StringVar(&sourceImage, "i", sourceImage, "alias for -image")

	flag.StringVar(&destination, "destination-repository", "",
		"destination repository which will be picked from \"config.json\" based on repository \"name\" or \"additionalNames\". "+
			"If starts with \"!\" repositories from \"config.json\" will be ignored, it will execute push to specified destination followed by \"!\".")
	flag.StringVar(&destination, "d", destination, "alias for -destination")

	flag.StringVar(&overrideTag, "override-tag", "", "override image tag")
	flag.StringVar(&overrideTag, "t", "", "alias for -override-tag")

	flag.BoolVar(&force, "force", false, "push image without asking for destination path verification")
	flag.BoolVar(&force, "f", force, "alias for -force")

	flag.Parse()

	if sourceImage == "" {
		log.Fatal("Must specify -image")
	}

	if containerTool == "" {
		log.Fatal("Must specify -container-tool")
	}

	if destination == "" {
		log.Fatal("Must specify -destination")
	}

	// Load config
	config, err := LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	// Generate destinationImage
	destinationImage, err := GenerateDestinationPathFromSourcePathAndConfig(sourceImage, destination, config)
	if err != nil {
		log.Fatal(err)
	}

	// Respect global destination mappings
	destinationImage = ApplyDestinationMapping(destinationImage, config.DestinationMapping)

	// Override tag
	if overrideTag != "" {
		destinationImage = OverrideTag(overrideTag, destinationImage)
	}

	// Ask user if destinationImage is correct
	if !force {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print(fmt.Sprintf("Generated destination image: %s\nDo you agree to tag & push it? [type y to confirm]: ", destinationImage))
		text, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(text)) != "y" {
			log.Fatal("Push aborted by user.")
		}
	}

	log.Println("Pulling image...")
	result, err := PullImage(containerTool, sourceImage)
	log.Println(result)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Tagging image...")
	result, err = TagImage(containerTool, sourceImage, destinationImage)
	log.Println(result)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Pushing image...")
	result, err = PushImage(containerTool, destinationImage)
	log.Println(result)
	if err != nil {
		log.Fatal(err)
	}
}

// Image destination logic

func GenerateDestinationPathFromSourcePathAndConfig(image string, destination string, config Config) (string, error) {
	imageParts := strings.Split(image, "/")

	if len(imageParts) < 2 {
		return GetDestination(destination, image, config)
	}

	for _, repo := range config.Repositories {
		imageWithoutRegistry, found := strings.CutPrefix(image, repo.GetRegistryPath()+"/")
		if found {
			return GetDestination(destination, imageWithoutRegistry, config)
		}
	}

	return "", RepositoryNotFoundForSourceError{image}
}

func GetDestination(destination string, imagePathWithoutRegistry string, config Config) (string, error) {
	if strings.HasPrefix(destination, "!") {
		return strings.Replace(destination, "!", "", 1), nil
	}

	for _, repo := range config.Repositories {
		if repo.Name == destination {
			return ApplyDestinationMapping(repo.GetRegistryPath()+"/"+imagePathWithoutRegistry, repo.DestinationMapping), nil
		}

		for _, additional := range repo.AdditionalNames {
			if additional == destination {
				return ApplyDestinationMapping(repo.GetRegistryPath()+"/"+imagePathWithoutRegistry, repo.DestinationMapping), nil
			}
		}

	}

	return "", RepositoryNotFoundForDestinationError{destination: destination}
}

func ApplyDestinationMapping(path string, mapping map[string]string) string {
	for source, dest := range mapping {
		if strings.Contains(path, source) {
			return strings.Replace(path, source, dest, 1)
		}
	}

	return path
}

func OverrideTag(tag string, image string) string {
	// Find the last colon
	if i := strings.LastIndex(image, ":"); i != -1 {
		// Check if there's a "/" after the colon, which means this colon might be part of the registry/namespace.
		if strings.Index(image[i:], "/") == -1 {
			return image[:i] + ":" + tag
		}
	}

	return image + ":" + tag
}

// Container Tool

func PullImage(containerTool string, image string) (string, error) {
	command := exec.Command(containerTool, "pull", image)
	out, err := command.CombinedOutput()

	return string(out), err
}

func TagImage(containerTool string, srcImage string, destImage string) (string, error) {
	command := exec.Command(containerTool, "tag", srcImage, destImage)
	out, err := command.CombinedOutput()
	return string(out), err
}

func PushImage(containerTool string, image string) (string, error) {
	command := exec.Command(containerTool, "push", image)
	out, err := command.CombinedOutput()
	return string(out), err
}

// Config Loader

func LoadConfig() (Config, error) {
	ex, err := os.Executable()
	if err != nil {
		return Config{}, err
	}

	exPath := filepath.Dir(ex)

	jsonBytes, err := os.ReadFile(exPath + "/config.json")

	if err != nil {
		return Config{}, err
	}

	config := Config{}
	err = json.Unmarshal(jsonBytes, &config)

	if err != nil {
		return Config{}, err
	}

	return config, nil
}
