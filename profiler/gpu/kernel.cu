// Including stuff - should be moved out to iostream
#include <cuda.h>
#include <stdio.h>
#include "profiler.h"


// 10 threads, 1 block 
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

	printf("Allocating");
	cudaMalloc((void **)&A_d, size);
	cudaMalloc((void **)&B_d, size);
	cudaMalloc((void **)&res_d, size);

	printf(" Done Allocating, now copying");	
	cudaMemcpy(A_d, A, size, cudaMemcpyHostToDevice);
	cudaMemcpy(B_d, B, size, cudaMemcpyHostToDevice);
	printf(" Done Copying , now setting config params");

	const unsigned int numThreads = 32;
	unsigned int numBlocks = 2;

	printf("Kernel time");
	vecAddKernel<<<numBlocks, numThreads>>>(A_d, B_d, res_d, n);

	cudaDeviceSynchronize();

	// Once the exec is done, we move it back from device to host
	cudaMemcpy(res, res_d, size, cudaMemcpyDeviceToHost);
	printf("haha done copying results?");

	cudaFree(A_d);
	cudaFree(B_d);
	cudaFree(res_d);
	printf("Done freeing mem from device?");
}


// Main fn will include InitTrace() and FinitTrace()
int main() {
	const int n = 16;

	float A[n];
	float B[n];
	float res[n];

	printf("Entered the main fn?");
	for (int i = 0; i < n; i += 1) {
		A[i] = float(i);
		B[i] = A[i] / 1000.0f;
	}

    InitTrace();     // Begin profiling
	vecAdd(A, B, res, n);
	printf("Done with vecadd, bitches");
    finitTrace();    // End profiling
	return 0;
}
