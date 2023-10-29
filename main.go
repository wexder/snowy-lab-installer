package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"text/template"
	"time"

	"golang.org/x/term"
)

func main() {
	if err := install(); err != nil {
		fmt.Printf("Installation failed error= %s\n", err.Error())
		os.Exit(1)
		return
	}
}

func install() error {
	fmt.Printf("Starting installer\n")
	err := checkRoot()
	if err != nil {
		return err
	}

	err = mountMnt()
	if err != nil {
		return err
	}

	fmt.Printf("\n\nDetected the following disks:\n\n")
	disks, err := getDisks()
	if err != nil {
		return err
	}

	index := askForDisk(len(disks))
	selectedDisk := disks[index]
	fmt.Printf("Selected %s disk\n", selectedDisk.name)

	hostname := askForHostname()
	fmt.Printf("Node hostname: %s\n", hostname)

	username := askForUsername()
	fmt.Printf("Your username: %s\n", username)

	password := askForPassword()
	fmt.Printf("Your password: ....! Just kidding\n")

	fmt.Printf("Proceeding will entail repartitioning and formatting /dev/%s.\n", selectedDisk.name)
	fmt.Printf("!!! ALL DATA ON /dev/%s WILL BE LOST !!!\n", selectedDisk.name)

	askToProceed()

	fmt.Printf("Ok, will begin installing in 10 seconds. Press Ctrl-C to cancel.\n")
	waitSeconds(10)

	err = runParted(selectedDisk)
	if err != nil {
		return fmt.Errorf("Failed running parted: %w", err)
	}

	waitForPartision(selectedDisk)

	err = makeBootPartision(selectedDisk)
	if err != nil {
		return fmt.Errorf("Failed making boot partisions: %w", err)
	}

	refreshBlockIndex(selectedDisk)

	err = mountNixosDisk()
	if err != nil {
		return fmt.Errorf("Failed mounting nixos disk: %w", err)
	}

	err = generateNixosConfig()
	if err != nil {
		return fmt.Errorf("Failed generating nixos config: %w", err)
	}

	err = generatePasswordFile(username, password)
	if err != nil {
		return fmt.Errorf("Failed generating user pass file: %w", err)
	}

	err = generateInstallationConfig(selectedDisk.name, hostname, username)
	if err != nil {
		return fmt.Errorf("Failed generating nixos install config: %w", err)
	}

	err = installNixos()
	if err != nil {
		return fmt.Errorf("Failed installing: %w", err)
	}

	err = removeInstallConfig()
	if err != nil {
		return fmt.Errorf("Failed removing installation config: %w", err)
	}

	err = generateSnowyLabConfig()
	if err != nil {
		return fmt.Errorf("Failed generating snowy lab config: %w", err)
	}

	// err = applySnowyLab()
	// if err != nil {
	// 	return fmt.Errorf("Failed applying snowy lab config: %w", err)
	// }

	return nil
}

func applySnowyLab() error {
	err := run("nixos-rebuild", "switch")
	if err != nil {
		return err
	}

	return nil
}

func removeInstallConfig() error {
	err := os.Remove("/mnt/etc/nixos/configuration.nix")
	if err != nil {
		return err
	}

	return nil
}

func installNixos() error {
	err := run("nixos-install", "--no-root-passwd")
	if err != nil {
		return err
	}

	return nil
}

func generatePasswordFile(username string, password string) error {
	fmt.Printf(">>> mkpasswd --method=sha-512\n")
	f, err := os.OpenFile(fmt.Sprintf("/mnt/etc/passwordFile-%s", username), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	command := exec.Command("mkpasswd", "--method=sha-512", password)
	command.Stdout = f
	command.Stderr = os.Stderr

	err = command.Run()
	if err != nil {
		return err
	}

	err = f.Sync()
	if err != nil {
		return err
	}

	return nil
}

//go:embed templates/configuration_install.nix.tmpl
var installTemplate []byte

func generateInstallationConfig(diskName string, hostname string, username string) error {
	tmpl, err := template.New("install").
		Parse(string(installTemplate))
	if err != nil {
		return err
	}

	f, err := os.Create("/mnt/etc/nixos/configuration.nix")
	if err != nil {
		return err
	}
	defer f.Close()

	err = tmpl.Execute(f, struct {
		Username string
		DiskName string
		Hostname string
		IsEfi    bool
	}{
		Username: username,
		DiskName: diskName,
		Hostname: hostname,
		IsEfi:    isEfi(),
	})
	if err != nil {
		return err
	}

	err = f.Sync()
	if err != nil {
		return err
	}

	return nil
}

//go:embed templates/configuration.nix.tmpl
var snowyConfTemplate []byte

//go:embed templates/flake.nix.tmpl
var flakeTemplate []byte

func generateSnowyLabConfig() error {
	tmpl, err := template.New("snowy-lab").
		Parse(string(snowyConfTemplate))
	if err != nil {
		return err
	}

	confF, err := os.Create("/mnt/etc/nixos/configuration.nix")
	if err != nil {
		return err
	}
	defer confF.Close()

	// Not necessary but ready for future
	err = tmpl.Execute(confF, struct{}{})
	if err != nil {
		return err
	}

	err = confF.Sync()
	if err != nil {
		return err
	}

	tmpl, err = template.New("snowy-lab").
		Parse(string(flakeTemplate))
	if err != nil {
		return err
	}

	flakeF, err := os.Create("/mnt/etc/nixos/flake.nix")
	if err != nil {
		return err
	}
	defer flakeF.Close()

	// Not necessary but ready for future
	err = tmpl.Execute(flakeF, struct{}{})
	if err != nil {
		return err
	}

	err = flakeF.Sync()
	if err != nil {
		return err
	}

	return nil
}

func generateNixosConfig() error {
	err := run("nixos-generate-config", "--root", "/mnt")
	if err != nil {
		return err
	}

	return nil
}

func mountNixosDisk() error {
	isEfi := isEfi()
	err := run("mount", "/dev/disk/by-label/nixos", "/mnt")
	if err != nil {
		return err
	}

	if isEfi {
		err := run("mkdir", "-p", "/mnt/boot")
		if err != nil {
			return err
		}
		err = run("mount", "/dev/disk/by-label/boot", "/mnt/boot")
		if err != nil {
			return err
		}
	}

	return nil
}

func refreshBlockIndex(disk disk) {
	for i := 0; i < 10; i++ {
		err := run("blockdev", "--rereadpt", fmt.Sprintf("/dev/%s", disk.name))
		if err != nil {
			continue
		}
		time.Sleep(1 * time.Second)
		_, err = os.Stat("/dev/disk/by-label/nixos")
		if err == nil {
			return
		}
	}

	fmt.Printf("WARNING: Failed to re-read the block index on /dev/%s. Things may break.", disk.name)
}

func makeBootPartision(disk disk) error {
	isEfi := isEfi()

	if isEfi {
		err := run("mkfs.fat", "-F", "32", "-n", "boot", fmt.Sprintf("/dev/%s", partitionName(disk.name, 1)))
		if err != nil {
			return err
		}
		err = run("mkfs.ext4", "-L", "nixos", fmt.Sprintf("/dev/%s", partitionName(disk.name, 2)))
		if err != nil {
			return err
		}
	} else {
		err := run("mkfs.ext4", "-L", "nixos", fmt.Sprintf("/dev/%s", partitionName(disk.name, 1)))
		if err != nil {
			return err
		}
	}
	return nil
}

func partitionName(diskName string, partition int32) string {
	if strings.HasPrefix(strings.ToLower(diskName), "sd") {
		return fmt.Sprintf("%s%d", diskName, partition)
	} else if strings.HasPrefix(strings.ToLower(diskName), "nvme") {
		return fmt.Sprintf("%sp%d", diskName, partition)
	} else {
		fmt.Printf("Warning: this type of device driver has not been thoroughly tested with nixos-up, and its partition naming scheme may differ from what we expect.. Type: %s\n", diskName)
		return fmt.Sprintf("%s%d", diskName, partition)
	}
}

func waitForPartision(disk disk) {
	for i := 0; i < 10; i++ {
		_, err := os.Stat(fmt.Sprintf("/dev/%s", partitionName(disk.name, 1)))
		if err == nil {
			return
		}

		time.Sleep(1 * time.Second)
	}

	fmt.Printf("WARNING: Waited for /dev/%s to show up but it never did. Things may break.\n", partitionName(disk.name, 1))
}

func waitSeconds(n int) {
	if n <= 0 {
		return
	}
	for i := 0; i < n; i++ {
		fmt.Printf("%d...", n-i)
		time.Sleep(1 * time.Second)
	}
	fmt.Println()
}

func runParted(selectedDisk disk) error {
	isEfi := isEfi()
	diskPath := fmt.Sprintf("/dev/%s", selectedDisk.name)

	if isEfi {
		fmt.Printf("Detected EFI/UEFI boot. Proceeding with a GPT partition scheme...\n")
		err := run("parted", diskPath, "-s", "--", "mklabel", "gpt")
		if err != nil {
			return err
		}
		// Create boot partition with first 512MiB.
		err = run("parted", diskPath, "-s", "--", "mkpart", "ESP", "fat32", "1MiB", "512MiB")
		if err != nil {
			return err
		}
		// Set the partition as bootable
		err = run("parted", diskPath, "-s", "--", "set", "1", "esp", "on")
		if err != nil {
			return err
		}
		// Create root partition after the boot partition.
		err = run("parted", diskPath, "-s", "--", "mkpart", "primary", "512MiB", "100%")
		if err != nil {
			return err
		}
	} else {
		fmt.Printf("Did not detect an EFI/UEFI boot. Proceeding with a legacy MBR partitioning scheme...\n")
		err := run("parted", diskPath, "-s", "--", "mklabel", "msdos")
		if err != nil {
			return err
		}
		err = run("parted", diskPath, "-s", "--", "mkpart", "primary", "1MiB", "100%")
		if err != nil {
			return err
		}

	}

	return nil
}

func isEfi() bool {
	f, err := os.Stat("/sys/firmware/efi")
	if err != nil {
		return false
	}

	return f.IsDir()
}

func run(cmd string, args ...string) error {
	fmt.Printf(">>> %s %s\n", cmd, strings.Join(args, " "))

	command := exec.Command(cmd, args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err := command.Run()
	if err != nil {
		return err
	}

	return nil
}

func checkRoot() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("snowy-lab-installer must be run as root!")
	}

	return nil
}

func mountMnt() error {
	cmd := exec.Command("mountpoint", "/mnt")
	err := cmd.Run()
	if err == nil {
		return fmt.Errorf("Something is already mount at /mnt!")
	}

	return nil
}

type disk struct {
	name    string
	rawSize string
	vendor  string
	model   string
}

func getDisks() ([]disk, error) {
	dir, err := os.ReadDir("/sys/block/")
	if err != nil {
		return nil, err
	}
	disks := []disk{}
	for _, entry := range dir {
		diskPath := path.Join("/sys/block", entry.Name(), "device")
		p, err := os.Stat(diskPath)
		if err != nil {
			continue
		}
		if !p.IsDir() {
			continue
		}

		vendor, _ := readFirstLine(path.Join(diskPath, "vendor"))
		model, _ := readFirstLine(path.Join(diskPath, "model"))
		rawSize, _ := readFirstLine(path.Join("/sys/block", entry.Name(), "size"))
		fmt.Printf("Disk: name= %-12s vendor= %-12s model= %-32s size= %-10s\n", entry.Name(), vendor, model, formatSize(rawSize))
		disks = append(disks, disk{
			name:    entry.Name(),
			rawSize: rawSize,
			vendor:  vendor,
			model:   model,
		})
	}
	return disks, nil
}

func askForDisk(diskCount int) int {
	fmt.Printf("Which disk number would you like to install onto (1-%d)?\n", diskCount)
	reader := bufio.NewReader(os.Stdin)
	// ReadString will block until the delimiter is entered
	input, err := reader.ReadString('\n')
	if err != nil {
		return askForDisk(diskCount)
	}

	// remove the delimeter from the string
	input = strings.TrimSuffix(input, "\n")
	index, err := strconv.ParseInt(input, 10, 64)
	if err != nil {
		fmt.Printf("Input must be a number\n")
		return askForDisk(diskCount)
	}
	if index > int64(diskCount) {
		fmt.Printf("Input must be between 1-%d\n", diskCount)
		return askForDisk(diskCount)
	}
	return int(index - 1)
}

func askToProceed() {
	fmt.Printf("Are you sure you'd like to proceed? If so, please type 'yes' in full, otherwise Ctrl-C: \n")
	reader := bufio.NewReader(os.Stdin)
	// ReadString will block until the delimiter is entered
	input, err := reader.ReadString('\n')
	if err != nil {
		askToProceed()
		return
	}

	// remove the delimeter from the string
	input = strings.TrimSuffix(input, "\n")
	if input == "yes" {
		return
	}

	askToProceed()
	return
}

func askForUsername() string {
	fmt.Printf("Username ?\n")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return askForUsername()
	}

	// remove the delimeter from the string
	input = strings.TrimSuffix(input, "\n")
	r, err := regexp.Compile(`^[a-z_][a-z0-9_-]*[\$]?$`)
	if err != nil {
		panic(err)
	}
	if !r.Match([]byte(input)) {
		fmt.Printf(`Usernames must begin with a lower case letter or an underscore,
    followed by lower case letters, digits, underscores, or dashes. They can end
    with a dollar sign.\n`)
		return askForUsername()
	}
	return input
}

func askForHostname() string {
	fmt.Printf("Hostname ?\n")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return askForHostname()
	}

	// TODO add validation
	return strings.TrimSuffix(input, "\n")
}

func askForPassword() string {
	fmt.Printf("User password ?\n")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return askForPassword()
	}
	fmt.Printf("Validate password ?\n")
	bytePasswordAgain, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return askForPassword()
	}
	if string(bytePassword) != string(bytePasswordAgain) {
		fmt.Printf("Password does not match\n")
		return askForPassword()
	}

	return string(bytePassword)
}

func formatSize(rawSize string) string {
	size, _ := strconv.ParseInt(rawSize, 10, 64)
	const unit = 1024
	sizeGB := float64(size) / 2 / unit / unit
	return fmt.Sprintf("%.3f GB total", sizeGB)
}

func readFirstLine(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	line, _, err := bufio.NewReader(f).ReadLine()
	if err != nil {
		return "", err
	}
	return string(line), nil
}
