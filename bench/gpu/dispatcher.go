package main

import (
	"fmt"
	"os"
	// "os/exec"
	"path/filepath"
)

// TODO: convertn the command to this bs !nvcc bench/gpu/nvidia/basic_vector.cu profiler/gpu/profiler.cpp -o profiler_app -Iprofiler/gpu -lcupti
// This dispatcher just prints out the respective file paths for now. Later on it will be integrated with C lib and CUDA kernels can be directly run
// from there


func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	currentDir, err := os.Getwd()
	checkErr(err)

	subDirs, err := os.ReadDir(currentDir)
	checkErr(err)


	for _, file := range subDirs {
		if file.IsDir() {
			fmt.Println("Sub dir:", file.Name())
			newPath := filepath.Join(currentDir, file.Name())

			subDirsContents, err := os.ReadDir(newPath)
			checkErr(err)

			fmt.Println("Contents in the", file.Name(), "folder:", subDirsContents)

			for _, subFile := range subDirsContents {
				cmd := fmt.Sprintf("nvcc -o profiler_app %s profiler.cpp -lcupti", subFile.Name())
				fmt.Println("command:", cmd)

				// TODO: weekend
				// cmd := exec.Command(cmd)
				// if err := cmd.Run(); err != nil {panic(err)}

			}
		}
	}
}
