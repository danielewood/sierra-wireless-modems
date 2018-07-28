## Flash using Sierra Wireless Linux Flashing Tool (fwdwl-lite)
### MC/EM 74XX Series
+ Stop and disable ModemManager during update process
    ```systemctl stop ModemManager
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
    --enable 1 \
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
    --enable 1 \
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
    --enable 1 \
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
