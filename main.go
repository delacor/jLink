package main

/*
#cgo CFLAGS: -Iheaders
#cgo LDFLAGS: -L${SRCDIR}/lib -Wl,-rpath,${SRCDIR}/lib -ljabra -l:libcurl.so.4

#include "Common.h"
#include "GoWrapper.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"log"
	"os"
	"time"
	"unsafe"
)

// sudo apt install libasound2 libcurl4
func main() {

	oldSettings, err := enableRawMode()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to enable raw mode:", err)
		return
	}
	terminalRestored := false
	restoreUI := func() {
		if terminalRestored {
			return
		}
		fmt.Print("\x1b[?25h") // Show cursor again
		restoreTerminal(oldSettings)
		terminalRestored = true
	}
	defer restoreUI()
	go startKeysPressedListener()

	appId := C.CString("JabraLink")
	C.Jabra_SetAppID(appId)
	defer C.free(unsafe.Pointer(appId))

	deviceCatalogueParams := (*C.DeviceCatalogue_params)(C.malloc(C.sizeof_DeviceCatalogue_params))
	*deviceCatalogueParams = C.DeviceCatalogue_params{
		delayInSecondsBeforeStartingRefresh:       1,
		refreshAtConnect:                          true,
		refreshAtStartup:                          true,
		refreshScope:                              1,
		fetchDataForUnknownDevicesInTheBackground: true,
		minimumAgeBeforeUpdate:                    24 * 60 * 60,
	}
	defer C.free(unsafe.Pointer(deviceCatalogueParams))

	configParams := (*C.Config_params)(C.malloc(C.sizeof_Config_params))
	*configParams = C.Config_params{
		deviceCatalogue_params: deviceCatalogueParams,
	}
	defer C.free(unsafe.Pointer(configParams))

	// Callback parameters: FirstScanForDevicesDoneFunc, DeviceAttachedFunc, DeviceRemovedFunc,
	// ButtonInDataRawHidFunc, ButtonInDataTranslatedFunc, nonJabraDeviceDetection, configParams
	if init := C.Jabra_InitializeV2(
		nil,                              // FirstScanForDevicesDoneFunc (not used here)
		(*[0]byte)(C.deviceAttachedFunc), // Callback for when a device is attached
		(*[0]byte)(C.deviceRemovedFunc),  // Callback for when a device is removed
		nil,                              // Callback for raw HID button input (not used here)
		nil,                              // Callback for translated button input (not used here)
		false,                            // nonJabraDeviceDetection (not used here)
		configParams,                     // Additional configuration parameters
	); !init {
		log.Println("Failed to initialize Jabra SDK")
		return
	}

	// The current callback behavior is inconsistent. While the charging status updates as expected,
	// the `levelInPercent` callback is sometimes delayed. This causes issues with timely updates.
	// We need to ensure that the callback is triggered in a more predictable and consistent manner.
	// C.Jabra_RegisterBatteryStatusUpdateCallbackV2((*[0]byte)(unsafe.Pointer(C.batteryStatusUpdate)))

	fmt.Print("\x1b[?25l") // Hide cursor
	clearScreen()
	startUi()

	restoreUI()
	stopBackgroundUpdates()
	fmt.Println("\n\nThank you for using jlink! (ʘ‿ʘ)╯")
	uninitializeWithTimeout(2 * time.Second)
}

func stopBackgroundUpdates() {
	stopBatteryStatusUpdates()
	stopPairingListUpdates()
}

func uninitializeWithTimeout(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		uninitialize()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		fmt.Fprintln(os.Stderr, "Timed out while uninitializing Jabra SDK; exiting anyway.")
	}
}
