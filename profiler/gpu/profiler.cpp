// This is the profiler for GPUs, make sure you have CUDA and CUPTI installed in
// your machine. This is a prototype, not fully planned and built out yet. To
// run this, I've included a toy vector addition kernel. You can run it using
// the following command: nvcc -o profiled_kernel kernel.cu profiler.cpp -lcupti
// -I/opt/cuda/extras/CUPTI/include -L/opt/cuda/extras/CUPTI/lib64  -lcupti
// Replace the path above with the path to your Cuda installation if it fails to
// compile citing absence of -lcupti
// ./profiled_kernel

// TODO: get rid of the memory bug
// TODO: add more metrics to be tracked

#include <cstdint>
#include <cstdlib>
#include <cuda.h>
#include <cupti.h>
#include <iostream>

// Defining buffer size
#define BUF_SIZE (32 * 1024)
#define ALIGN_SIZE (8)

// Timestamp at trace initialization time. Used to normalized other
static uint64_t startTimestamp;

// macro for calling a fn? I'm not sure what this does
#define CUPTI_CALL(call)                                                       \
  do {                                                                         \
    CUptiResult _status = call;                                                \
    if (_status != CUPTI_SUCCESS) {                                            \
      const char *errstr;                                                      \
      cuptiGetResultString(_status, &errstr);                                  \
      fprintf(stderr, "%s:%d: error: function %s failed with error %s.\n",     \
              __FILE__, __LINE__, #call, errstr);                              \
      exit(-1);                                                                \
    }                                                                          \
  } while (0)

// Defining the handler for memory
static const char *getMemKind(CUpti_ActivityMemcpyKind kind) {
  switch (kind) {
  case CUPTI_ACTIVITY_MEMCPY_KIND_HTOD:
    return "Host to Device";
    break;
  case CUPTI_ACTIVITY_MEMCPY_KIND_DTOH:
    return "Device to Host";
    break;
  default:
    return "Unknown mem alloc";
    break;
  }
  return "<unknown>";
}

// We can call this the entry point to the profiler, here we get all
// types of attributes and activities and log them.
static void printActivity(CUpti_Activity *record) {
  switch (record->kind) {

    // CUPTI_ACTIVITY_KIND_DEVICE is for the actual GPU device itself.
    // We are logging the followign: device ID,
    //                               global memory (DRAM) bandwidth and size
    //                               Number of streaming multiprocessors
    //                               L2 Cache size

  case CUPTI_ACTIVITY_KIND_DEVICE: {
    CUpti_ActivityDevice5 *device = (CUpti_ActivityDevice5 *)record;

    uint64_t gBand = device->globalMemoryBandwidth;
    uint64_t gMemSize = device->globalMemorySize;
    uint64_t l2size = device->l2CacheSize;
    uint64_t numProcessors = device->numMultiprocessors;
    uint32_t deviceId = device->id;

    std::cout << "[Device Info]" << std::endl;
    std::cout << "  Device ID              : " << deviceId << std::endl;
    std::cout << "  Global Mem Bandwidth   : " << gBand / 1e9 << " GB/s"
              << std::endl;
    std::cout << "  Global Mem Size        : " << gMemSize / (1024 * 1024)
              << " MB" << std::endl;
    std::cout << "  L2 Cache Size          : " << l2size / 1024 << " KB"
              << std::endl;
    std::cout << "  Multiprocessors        : " << numProcessors << std::endl;
    break;
  }

    // CUPTI_ACTIVITY_KIND_MEMCPY tracks memory transfers between host and
    // device and we track the following ones for simplicity; bytes deviceId
    // contId
    // This section is not done yet. for some reason its not tracking the number
    // of bytes copied but is able to get the type of memcpy happening

  case CUPTI_ACTIVITY_KIND_MEMCPY: {
    CUpti_ActivityMemcpy5 *memcpyRec = (CUpti_ActivityMemcpy5 *)record;

    uint64_t bytes = memcpyRec->bytes;
    uint64_t deviceId = memcpyRec->deviceId;
    uint64_t contId = memcpyRec->contextId;
    uint64_t streamId = memcpyRec->streamId;

    CUpti_ActivityMemcpyKind kind =
        (CUpti_ActivityMemcpyKind)memcpyRec->copyKind;

    std::cout << "[Memcpy]" << std::endl;
    std::cout << "  Kind             : " << getMemKind(kind) << std::endl;
    std::cout << "  Bytes            : " << bytes << std::endl;
    std::cout << "  Device ID        : " << deviceId << std::endl;
    std::cout << "  Context ID       : " << contId << std::endl;
    std::cout << "  Stream ID        : " << streamId << std::endl;
    std::cout << "  Start Time (ns)  : " << memcpyRec->start - startTimestamp
              << std::endl;
    std::cout << "  End Time (ns)    : " << memcpyRec->end - startTimestamp
              << std::endl;
    std::cout << "  Duration (μs)    : "
              << (memcpyRec->end - memcpyRec->start) / 1000.0 << std::endl;
    break;
  }

  default:
    std::cout << "[Unknown Activity Type] kind=" << record->kind << std::endl;
    break;
  }
}

// These are callbacks for requesting and completing buffers.
// Each buffer records the information that the printActivity fn spits out
static void CUPTIAPI bufferRequested(uint8_t **buffer, size_t *size,
                                     size_t *maxNumRecords) {
  *size = BUF_SIZE;
  uint8_t *rawBuffer = (uint8_t *)malloc(*size + ALIGN_SIZE);

  *buffer = rawBuffer;
  *maxNumRecords = 0;
}

static void CUPTIAPI bufferCompleted(CUcontext ctx, uint32_t streamId,
                                     uint8_t *buffer, size_t size,
                                     size_t validSize) {
  CUptiResult status;
  CUpti_Activity *record = NULL;

  do {
    status = cuptiActivityGetNextRecord(buffer, validSize, &record);
    if (status == CUPTI_SUCCESS) {
      printActivity(record);
    } else if (status == CUPTI_ERROR_MAX_LIMIT_REACHED) {
      break;
    } else {
      CUPTI_CALL(status);
    }
  } while (1);

  size_t dropped;
  CUPTI_CALL(cuptiActivityGetNumDroppedRecords(ctx, streamId, &dropped));
  if (dropped != 0) {
    printf("Dropped %u activity records\n", (unsigned int)dropped);
  }

  free(buffer);
}

// Init trace for starting tracing. We establish what metrics to be traced.
void InitTrace() {

  // Initialize the device and memory attribute tracking
  CUPTI_CALL(cuptiActivityEnable(CUPTI_ACTIVITY_KIND_DEVICE));
  CUPTI_CALL(cuptiActivityEnable(CUPTI_ACTIVITY_KIND_MEMCPY));

  CUPTI_CALL(cuptiActivityRegisterCallbacks(bufferRequested, bufferCompleted));
  CUPTI_CALL(cuptiGetTimestamp(&startTimestamp));
}

void finitTrace() { CUPTI_CALL(cuptiActivityFlushAll(0)); }
