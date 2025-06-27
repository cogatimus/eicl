// Including stuff - should be moved out to iostream
#include <cuda.h>
#include <iostream>

// How to include this -- idk
#include "profiler/gpu/profiler.h"

__global__ void vecAddKernel(float *A, float *B, float *res, int n) {
	
	// Block Index - blockIdx.x
	// Thread Index - threadIdx.x
	int i = threadIdx.x + blockDim.x * blockIdx.x;

	if (i < n) {
		res[i] = A[i] + B[i];
	}

}



// THis will be run on your CPU
void vecAdd(float *A, float *B, float *res, int n) {
	float *A_d, *B_d, *res_d;
	size_t size = n * sizeof(float);

	cudaMalloc((void **)&A_d, size);
	cudaMalloc((void **)&B_d, size);
	cudaMalloc((void **)&res_d, size);

	cudaMemcpy(A_d, A, size, cudaMemcpyHostToDevice);
	cudaMemcpy(B_d, B, size, cudaMemcpyHostToDevice);

	const unsigned int numThreads = 32;
	unsigned int numBlocks = 2;

	vecAddKernel<<<numBlocks, numThreads>>>(A_d, B_d, res_d, n);

    // Tell the CPU that the kernel has finished and we can copy things back
	cudaDeviceSynchronize();

	// Once the exec is done, we move it back from device to host
	cudaMemcpy(res, res_d, size, cudaMemcpyDeviceToHost);

	cudaFree(A_d);
	cudaFree(B_d);
	cudaFree(res_d);
}


// Main fn will include InitTrace() and FinitTrace()
int main() {
	const int n = 16;

	float *A = new float[n];
	float *B = new float[n];
	float *res = new float[n];

	for (int i = 0; i < n; i += 1) {
		A[i] = float(i);
		B[i] = A[i] / 1000.0f;
	}

    InitTrace();     // Begin profiling
	vecAdd(&A, &B, &res, n);
    finitTrace();    // End profiling
	return 0;
}
