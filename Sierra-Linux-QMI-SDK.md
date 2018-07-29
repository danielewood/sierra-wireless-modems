## Flash using Sierra Wireless Linux Flashing Tool (fwdwl-lite)
### MC/EM 74XX Series
+ Stop and disable ModemManager during update process
    ```
    systemctl stop ModemManager
    systemctl disable ModemManager
    ```
+ Download and unzip the latest Generic Firmware (Linux)
    + [SWI9X30C_02.24.05.06_Generic_002.026_000.zip](https://source.sierrawireless.com/resources/airprime/minicard/74xx/airprime-em_mc74xx-approved-fw-packages/)
    + `unzip SWI9X30C_02.24.05.06_Generic_002.026_000.zip`
+ Download and Extract the latest Linux QMI SDK Software (fwdwl-litehostx86_64)
    + [SLQS04.00.15-lite.bin.tar.gz](https://source.sierrawireless.com/resources/airprime/software/linux-qmi-sdk-software-latest/)
    + `tar --extract --strip-components 3 --file SLQS04.00.15-lite.bin.tar.gz SampleApps/lite-fw-download/bin/fwdwl-litehostx86_64`
+ Flash the Modem `(if --dmreset doesnt work, try removing it)`:
    ```
    devpath=`ls /dev | grep -i -E 'cdc-wdm|qcqmi'`
    devtype=`expr "$devpath" : '\(cdc-wdm\|qcqmi\)[0-9]$'`
    case $devtype in
        cdc-wdm) devtype="MBIM" ;;
        qcqmi) devtype="QMI" ;;
        *) printf "Unknown Device Type = $devtype\r\nDevice Path = /dev/$devpath\r\n"; exit
    esac
    printf "Device Type = $devtype\r\nDevice Path = /dev/$devpath\r\n"
    
    ./fwdwl-litehostx86_64 \
    --devmode $devtype  \
    --devpath /dev/$devpath \
    --modelfamily 3 \
    --logfile "fwdwl-lite-$devpath.log" \
    --enable \
    --fwpath "./" \
    --dmreset
    ```
+ Re-enable and start ModemManager
    ```
    systemctl enable ModemManager
    systemctl start ModemManager
    ```
---
### EM7565 from Version <01.05.01.00 (Release <9) to Latest Release (9+)
+ **WARNING: Completely untested and theoretical based on research and [others experiences](https://forum.sierrawireless.com/t/solved-em7565-firmware-update-from-01-00-02-00-to-01-07-00-00/13010/19).**
+ If the EM7565 is below Release 9, you must first upgrade to Release 9. This intermediate step is required due to NVU signing implemented by Sierra Wireless. Due to the same changes, you must also flash the latest firmware twice to enable the modem to exit Low Power Mode.
1. Stop and disable ModemManager during update process
    ```
    systemctl stop ModemManager
    systemctl disable ModemManager
    ```
2. Create folders to hold firmware versions
    + `mkdir swi_fw0105 swi_fwlatest`
3. Download and unzip the 01.05.01.00 Generic Firmware (Linux)
    + [SWI9X50C_01.05.01.00_00_GENERIC_001.028_000.zip](https://source.sierrawireless.com/resources/airprime/minicard/75xx/fw/swi9x50c_01,-d-,05,-d-,01,-d-,00_00_generic_001,-d-,028_000/)
    + `unzip SWI9X50C_01.05.01.00_00_GENERIC_001.028_000.zip -d ./swi_fw0105`
4. Download and unzip the latest 7565 Generic Firmware (Linux)
    + Currently: [SWI9X50C_01.07.02.00_GENERIC_002.004_000.zip](https://source.sierrawireless.com/resources/airprime/minicard/75xx/airprime-em_mc75xx-approved-fw-packages/)
    + `unzip SWI9X50C_01.07.02.00_GENERIC_002.004_000.zip -d ./swi_fwlatest`
5. Download and Extract the latest Linux QMI SDK Software (fwdwl-litehostx86_64)
    + [SLQS04.00.15-lite.bin.tar.gz](https://source.sierrawireless.com/resources/airprime/software/linux-qmi-sdk-software-latest/)
    + `tar --extract --strip-components 3 --file SLQS04.00.15-lite.bin.tar.gz SampleApps/lite-fw-download/bin/fwdwl-litehostx86_64`
6. Flash to SWI9X50C_01.05.01.00_00_GENERIC_001.028_000 `(if --dmreset doesnt work, try removing it)`:
    ```
    devpath=`ls /dev | grep -i -E 'cdc-wdm|qcqmi'`
    devtype=`expr "$devpath" : '\(cdc-wdm\|qcqmi\)[0-9]$'`
    case $devtype in
        cdc-wdm) devtype="MBIM" ;;
        qcqmi) devtype="QMI" ;;
        *) printf "Unknown Device Type = $devtype\r\nDevice Path = /dev/$devpath\r\n"; exit
    esac
    printf "Device Type = $devtype\r\nDevice Path = /dev/$devpath\r\n"
    
    ./fwdwl-litehostx86_64 \
    --devmode $devtype  \
    --devpath /dev/$devpath \
    --modelfamily 4 \
    --logfile "fwdwl-lite-$devpath.log" \
    --enable \
    --fwpath "./swi_fw0105/" \
    --dmreset
    ```
7. Flash to **Latest Firmware**  `(if --dmreset doesnt work, try removing it)`:
    ```
    devpath=`ls /dev | grep -i -E 'cdc-wdm|qcqmi'`
    devtype=`expr "$devpath" : '\(cdc-wdm\|qcqmi\)[0-9]$'`
    case $devtype in
        cdc-wdm) devtype="MBIM" ;;
        qcqmi) devtype="QMI" ;;
        *) printf "Unknown Device Type = $devtype\r\nDevice Path = /dev/$devpath\r\n"; exit
    esac
    printf "Device Type = $devtype\r\nDevice Path = /dev/$devpath\r\n"
    
    ./fwdwl-litehostx86_64 \
    --devmode $devtype  \
    --devpath /dev/$devpath \
    --modelfamily 4 \
    --logfile "fwdwl-lite-$devpath.log" \
    --enable \
    --fwpath "./swi_fwlatest/" \
    --dmreset
    ```
8. Repeat **Step 7** to Flash the **Latest Firmware** a second time.
9. Re-enable and start ModemManager
    ```
    systemctl enable ModemManager
    systemctl start ModemManager
    ```
---
### fwdwl-lite Help File
```
./fwdwl-litehostx86_64 --help

App usage:

  <appName> -c <QMI/MBIM mode> -d <QDL Port> -p <QMI/MBIM Device path> -f <FW path> -h

  -c  --devmode
         Specifies the mode of the device, MBIM or QMI

        Defaults to QMI if not specified

  -d  --qdl
         Specifies the QDL port when modem switches to BOOT and HOLD mode to download firmware.

        For example: -d /dev/ttyUSB0

        Defaults to /dev/ttyUSB0 if not specified

  -p  --devpath
         Specifies the QMI or MBIM device

        Defaults to /dev/qcqmi0 for QMI and /dev/cdc-wdm0 for MBIM

  -f  --fwpath [folder to firmware images]
        This specifies the folder location of the firmware images. This option is mandatory.
        - 9x30: Specify the path containing a carrier FW package (.cwe and .nvu) or an OEM PRI (.nvu)

  -i  --ignore crash state checking or not.Default value is 0 means crash state checking is required
          - 0: crash state checking required (default value)
          - 1: ignore crash state checking

  -l  --logfile
        Specific custom log path.

  -b  --blocksize
        File Read Block Size.

  -m  --modelfamily
          - 0: Auto Detect (default value)
          - 1: 9x15 Family
          - 2: WP9x15 Family
          - 3: 9x30 Family
          - 4: 9x07 or 9x50 Family
          - 5: 9x06 Family

  -h  --help
        This option prints the usage instructions.

  -e  --enable/disable debug logs
           - 0 : Debug logs disable.
           - 1 : Debug logs enable.

 -r --dmreset reset modem using DM command
 This operation only support when modem in App mode.
 Don't use this option when modem is already in QDL mode.
 This option should not be used in normal download operation.
 Modem will not be reset on QDL mode
```
---
### fwdld Help File
```
./fwdldhostx86_64 --help

App usage:

  <appName> -s <sdk_path> -d [9x00/9x15/g3k] -p [pathname] -m [modem_index] -h

  -s  --sdkpath
         Specifies the path to the slqssdk. This is a mandatory parameter.

  -m  --modem
         Specifies the modem index. This is a optional parameter.

        Default to zero if not specified, that is first modem

  -u  --serial number or usb path
         Specifies the usb path. This is a optional parameter.

  -d  --device [9x00/9x15/9x30/g3k]
        Specifies the device type. Must be lower cases. This option is optional.
        If this option is omitted, 9x15 is the default setting.
          - 9x00: This is a 9x00 based device.
          - 9x15: This is a 9x15 based device.
          - 9x30: This is a 9x30 based device.
          - g3k:  This is a Gobi device.
          - 9x07: This is a 9x07 based device.
          - 9x50: This is a 9x50 based device.
          - 9x06: This is a 9x06 based device.

  -p  --path [folder to firmware images]
        This specifies the folder location of the firmware images. This option is mandatory.
        Usage of this parameter depends on type of device.
          - 9x00: Specify the path containing the image.
          - 9x15: Specify the path containing a combined image or separate images.
          - 9x30: Specify the path containing a combined image or separate images.
          - g3k: Should be in format <file path>/x , x should be a number,
                 and this folder should contain both AMSS and UQCN.
          - 9x07: Specify the path containing the image.
          - 9x50: Specify the path containing the image.
          - 9x06: Specify the path containing the image.

  -k  --kill kill SDK process on exits
  -f  --File path
  -i  --ignore crash state checking or not.Default value is 0 means crash state checking is required
          - 0: crash state checking required (default value)
          - 1: ignore crash state checking
  -l  --slot index of modem, this option is only available for EM74xx/MC74xx
  -h  --help
        This option prints the usage instructions.
```
---
### Binary Files Listing of SLQS04.00.15-lite.bin.tar.gz
```
root@ubuntu:/tmp# tar xf SLQS04.00.15-lite.bin.tar.gz
root@ubuntu:/tmp# find -type f -executable -exec file -i '{}' \; | grep 'x-executable; charset=binary'
./SampleApps/lite-qmi-demo/bin/packingdemoppc
./SampleApps/lite-qmi-demo/bin/packingdemohostx86_64
./SampleApps/lite-qmi-demo/bin/packingdemohosti686
./SampleApps/lite-qmi-demo/bin/packingdemoarm
./SampleApps/lite-qmi-demo/bin/packingdemoarm64
./SampleApps/lite-fw-download/bin/fwdwl-litehosti686
./SampleApps/lite-fw-download/bin/fwdwl-liteppc
./SampleApps/lite-fw-download/bin/fwdwl-litehostx86_64
./SampleApps/lite-fw-download/bin/fwdwl-litearm64
./SampleApps/lite-fw-download/bin/fwdwl-litearm
```
---
### Binary Files Listing of SLQS04.00.15.bin.tar.gz
```
root@ubuntu:/tmp/tmp1/tmp# tar xf SLQS04.00.15.bin.tar.gz
root@ubuntu:/tmp/tmp1/tmp# find -type f -executable -exec file -i '{}' \; | grep 'x-executable; charset=binary'
./SampleApps/Firmware_Download/bin/fwdldmips
./SampleApps/Firmware_Download/bin/fwdldhostx86_64
./SampleApps/Firmware_Download/bin/fwdldarm64
./SampleApps/Firmware_Download/bin/fwdldarm
./SampleApps/Firmware_Download/bin/fwdldmipsel
./SampleApps/Firmware_Download/bin/fwdldhosti686
./SampleApps/Firmware_Download/bin/fwdldppc
./SampleApps/Connection_Manager/bin/connectionmgrarm64
./SampleApps/Connection_Manager/bin/connectionmgrmips
./SampleApps/Connection_Manager/bin/connectionmgrmipsel
./SampleApps/Connection_Manager/bin/connectionmgrarm
./SampleApps/Connection_Manager/bin/connectionmgrhostx86_64
./SampleApps/Connection_Manager/bin/connectionmgrhosti686
./SampleApps/Connection_Manager/bin/connectionmgrppc
./SampleApps/Gobi_Image_Management/bin/gobiimgmgmtmips
./SampleApps/Gobi_Image_Management/bin/gobiimgmgmtmipsel
./SampleApps/Gobi_Image_Management/bin/gobiimgmgmthosti686
./SampleApps/Gobi_Image_Management/bin/gobiimgmgmtppc
./SampleApps/Gobi_Image_Management/bin/gobiimgmgmtarm
./SampleApps/Gobi_Image_Management/bin/gobiimgmgmtarm64
./SampleApps/Gobi_Image_Management/bin/gobiimgmgmthostx86_64
./SampleApps/SLQS_Tutorial_Application/bin/slqstutorialarm64
./SampleApps/SLQS_Tutorial_Application/bin/slqstutorialhostx86_64
./SampleApps/SLQS_Tutorial_Application/bin/slqstutorialhosti686
./SampleApps/SLQS_Tutorial_Application/bin/slqstutorialppc
./SampleApps/SLQS_Tutorial_Application/bin/slqstutorialmipsel
./SampleApps/SLQS_Tutorial_Application/bin/slqstutorialarm
./SampleApps/SLQS_Tutorial_Application/bin/slqstutorialmips
./SampleApps/MC7xxx_Image_Management/bin/mc7xxximgmgmtmipsel
./SampleApps/MC7xxx_Image_Management/bin/mc7xxximgmgmtppc
./SampleApps/MC7xxx_Image_Management/bin/mc7xxximgmgmtarm
./SampleApps/MC7xxx_Image_Management/bin/mc7xxximgmgmtarm64
./SampleApps/MC7xxx_Image_Management/bin/mc7xxximgmgmtmips
./SampleApps/MC7xxx_Image_Management/bin/mc7xxximgmgmthostx86_64
./SampleApps/MC7xxx_Image_Management/bin/mc7xxximgmgmthosti686
./SampleApps/SMS_Application/bin/SMSSampleApparm64
./SampleApps/SMS_Application/bin/SMSSampleApphostx86_64
./SampleApps/SMS_Application/bin/SMSSampleApparm
./SampleApps/SMS_Application/bin/SMSSampleAppmips
./SampleApps/SMS_Application/bin/SMSSampleAppmipsel
./SampleApps/SMS_Application/bin/SMSSampleApphosti686
./SampleApps/SMS_Application/bin/SMSSampleAppppc
./SampleApps/CallHandling_Application/bin/callhandlingppc
./SampleApps/CallHandling_Application/bin/callhandlingarm
./SampleApps/CallHandling_Application/bin/callhandlingmips
./SampleApps/CallHandling_Application/bin/callhandlingarm64
./SampleApps/CallHandling_Application/bin/callhandlinghosti686
./SampleApps/CallHandling_Application/bin/callhandlingmipsel
./SampleApps/CallHandling_Application/bin/callhandlinghostx86_64
./SampleApps/LOC_Service/bin/locservicemipsel
./SampleApps/LOC_Service/bin/locservicemips
./SampleApps/LOC_Service/bin/locserviceppc
./SampleApps/LOC_Service/bin/locservicehosti686
./SampleApps/LOC_Service/bin/locservicearm64
./SampleApps/LOC_Service/bin/locservicearm
./SampleApps/LOC_Service/bin/locservicehostx86_64
./SampleApps/SWIOMA_Application/bin/SWIOMASampleApparm64
./SampleApps/SWIOMA_Application/bin/SWIOMASampleApparm
./SampleApps/SWIOMA_Application/bin/SWIOMASampleApphosti686
./SampleApps/SWIOMA_Application/bin/SWIOMASampleApphostx86_64
./SampleApps/SWIOMA_Application/bin/SWIOMASampleAppmips
./SampleApps/SWIOMA_Application/bin/SWIOMASampleAppmipsel
./SampleApps/SWIOMA_Application/bin/SWIOMASampleAppppc
./SampleApps/PDS_Services/bin/pdsservicesarm64
./SampleApps/PDS_Services/bin/pdsservicesmipsel
./SampleApps/PDS_Services/bin/pdsservicesmips
./SampleApps/PDS_Services/bin/pdsserviceshosti686
./SampleApps/PDS_Services/bin/pdsserviceshostx86_64
./SampleApps/PDS_Services/bin/pdsservicesarm
./SampleApps/PDS_Services/bin/pdsservicesppc
./SampleApps/AirVantageAgent/build.default/runtime/bin/luasignalcmd
./SampleApps/AirVantageAgent/build.default/runtime/bin/agent
./SampleApps/AirVantageAgent/build.default/runtime/bin/slqssdk
./SampleApps/AirVantageAgent/build.default/runtime/bin/appmon_daemon
./SampleApps/AirVantageAgent/build.default/runtime/bin/slqs_fw_update
./SampleApps/AirVantageAgent/build.default/CMakeFiles/2.8.12.2/CMakeDetermineCompilerABI_CXX.bin
./SampleApps/AirVantageAgent/build.default/CMakeFiles/2.8.12.2/CompilerIdCXX/a.out
./SampleApps/AirVantageAgent/build.default/CMakeFiles/2.8.12.2/CMakeDetermineCompilerABI_C.bin
./SampleApps/AirVantageAgent/build.default/CMakeFiles/2.8.12.2/CompilerIdC/a.out
./SampleApps/AirVantageAgent/bin/start_sdkmipsel
./SampleApps/AirVantageAgent/build.arm/runtime/bin/luasignalcmd
./SampleApps/AirVantageAgent/build.arm/runtime/bin/agent
./SampleApps/AirVantageAgent/build.arm/runtime/bin/slqssdk
./SampleApps/AirVantageAgent/build.arm/runtime/bin/appmon_daemon
./SampleApps/AirVantageAgent/build.arm/runtime/bin/slqs_fw_update
./SampleApps/AirVantageAgent/build.arm/CMakeFiles/2.8.12.2/CMakeDetermineCompilerABI_CXX.bin
./SampleApps/AirVantageAgent/build.arm/CMakeFiles/2.8.12.2/CompilerIdCXX/a.out
./SampleApps/AirVantageAgent/build.arm/CMakeFiles/2.8.12.2/CMakeDetermineCompilerABI_C.bin
./SampleApps/AirVantageAgent/build.arm/CMakeFiles/2.8.12.2/CompilerIdC/a.out
./tools/logging/ramdump-legacy/bin/ramdumptooli386
./tools/logging/dm/bin/remserialppc
./tools/logging/dm/bin/remserialarm
./tools/logging/dm/bin/remserialhosti686
./tools/logging/dm/bin/remserialmips
./tools/logging/dm/bin/remserialmipsel
./tools/logging/dm/bin/split-sqfarm64
./tools/logging/dm/bin/split-sqfi386
./tools/logging/dm/bin/split-sqfarm
./tools/logging/dm/bin/split-sqfhosti686
./tools/logging/dm/bin/split-sqfppc
./tools/logging/dm/bin/split-sqfmips
./tools/logging/dm/bin/remseriali386
./tools/logging/dm/bin/split-sqfmipsel
./tools/logging/dm/bin/remserialarm64
./tools/logging/ramdump/bin/ramdumptool_mips
./tools/logging/ramdump/bin/ramdumptool_arm
./tools/logging/ramdump/bin/ramdumptool_hosti686
./tools/logging/ramdump/bin/ramdumptool_i386
./tools/logging/ramdump/bin/ramdumptool_arm64
./tools/logging/ramdump/bin/ramdumptool_ppc
./tools/logging/ramdump/bin/ramdumptool_mipsel
./pkgs/qa/qatestppc
./pkgs/qa/qatestmipsel
./pkgs/qa/qatestmips
./pkgs/qa/qatesthostx86_64
./pkgs/qa/qatesthosti686
./pkgs/qa/qatestarm64
./pkgs/qa/qatestarm
./build/bin/arm/slqssdk
./build/bin/mipsel/slqssdk
./build/bin/mips/slqssdk
./build/bin/arm64/slqssdk
./build/bin/hosti686/slqssdk
./build/bin/hostx86_64/slqssdk
./build/bin/ppc/slqssdk
```
