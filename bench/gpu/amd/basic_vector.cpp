#include <hip/hip_runtime.h>
#include <iostream>

__global__ void hello_world_kernel() {
    unsigned int thread_idx = threadIdx.x;
    unsigned int block_idx = blockIdx.x;

    std::cout << "Hello world from the GPU" << std::endl;
}


int main()
{
    // Launch the kernel.
    helloworld_kernel<<<dim3(2), // 3D grid specifying number of blocks to launch: (2, 1, 1)
                        dim3(2), // 3D grid specifying number of threads to launch: (2, 1, 1)
                        0, // number of bytes of additional shared memory to allocate
                        hipStreamDefault // stream where the kernel should execute: default stream
                    >>>();

}