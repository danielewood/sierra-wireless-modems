### Flash using Sierra Wireless Linux Flashing Tool (fwdwl-lite)
+ Download and unzip the latest Generic Firmware (Linux)
    + [SWI9X30C_02.24.05.06_Generic_002.026_000.zip](https://source.sierrawireless.com/resources/airprime/minicard/74xx/airprime-em_mc74xx-approved-fw-packages/)
    + `unzip SWI9X30C_02.24.05.06_Generic_002.026_000.zip`
+ Download and Extract the latest Linux QMI SDK Software (fwdwl-litehostx86_64)
    + [SLQS04.00.15-lite.bin.tar.gz](https://source.sierrawireless.com/resources/airprime/software/linux-qmi-sdk-software-latest/)
    + `tar --extract --strip-components 3 --file SLQS04.00.15-lite.bin.tar.gz SampleApps/lite-fw-download/bin/fwdwl-litehostx86_64`
+ For MBIM Modems `(if --dmreset doesnt work, try removing it)`:
    ```
    devpath=`ls /dev | grep cdc-wdm`
    ./fwdwl-litehostx86_64 \
    --devmode MBIM  \
    --devpath /dev/$devpath \
    --dmreset \
    --fwpath ./
    ```
+ For QMI Modems `(if --dmreset doesnt work, try removing it)`:
    ```
    devpath=`ls /dev | grep qcqmi`
    ./fwdwl-litehostx86_64 \
    --devmode QMI  \
    --devpath /dev/$devpath \
    --dmreset \
    --fwpath ./
    ```
---
### Binary Files Listing of SLQS04.00.15-lite.bin.tar.gz
```
root@ubuntu:/tmp# tar xf SLQS04.00.15-lite.bin.tar.gz
root@ubuntu:/tmp# find -type f -executable -exec file -i '{}' \; | grep 'x-executable; charset=binary'
./SampleApps/lite-qmi-demo/bin/packingdemoppc: application/x-executable; charset=binary
./SampleApps/lite-qmi-demo/bin/packingdemohostx86_64: application/x-executable; charset=binary
./SampleApps/lite-qmi-demo/bin/packingdemohosti686: application/x-executable; charset=binary
./SampleApps/lite-qmi-demo/bin/packingdemoarm: application/x-executable; charset=binary
./SampleApps/lite-qmi-demo/bin/packingdemoarm64: application/x-executable; charset=binary
./SampleApps/lite-fw-download/bin/fwdwl-litehosti686: application/x-executable; charset=binary
./SampleApps/lite-fw-download/bin/fwdwl-liteppc: application/x-executable; charset=binary
./SampleApps/lite-fw-download/bin/fwdwl-litehostx86_64: application/x-executable; charset=binary
./SampleApps/lite-fw-download/bin/fwdwl-litearm64: application/x-executable; charset=binary
./SampleApps/lite-fw-download/bin/fwdwl-litearm: application/x-executable; charset=binary
```


---
### Binary Files Listing of SLQS04.00.15.bin.tar.gz
```
root@ubuntu:/tmp/tmp1/tmp# tar xf SLQS04.00.15.bin.tar.gz
root@ubuntu:/tmp/tmp1/tmp# find -type f -executable -exec file -i '{}' \; | grep 'x-executable; charset=binary'
./SampleApps/Firmware_Download/bin/fwdldmips: application/x-executable; charset=binary
./SampleApps/Firmware_Download/bin/fwdldhostx86_64: application/x-executable; charset=binary
./SampleApps/Firmware_Download/bin/fwdldarm64: application/x-executable; charset=binary
./SampleApps/Firmware_Download/bin/fwdldarm: application/x-executable; charset=binary
./SampleApps/Firmware_Download/bin/fwdldmipsel: application/x-executable; charset=binary
./SampleApps/Firmware_Download/bin/fwdldhosti686: application/x-executable; charset=binary
./SampleApps/Firmware_Download/bin/fwdldppc: application/x-executable; charset=binary
./SampleApps/Connection_Manager/bin/connectionmgrarm64: application/x-executable; charset=binary
./SampleApps/Connection_Manager/bin/connectionmgrmips: application/x-executable; charset=binary
./SampleApps/Connection_Manager/bin/connectionmgrmipsel: application/x-executable; charset=binary
./SampleApps/Connection_Manager/bin/connectionmgrarm: application/x-executable; charset=binary
./SampleApps/Connection_Manager/bin/connectionmgrhostx86_64: application/x-executable; charset=binary
./SampleApps/Connection_Manager/bin/connectionmgrhosti686: application/x-executable; charset=binary
./SampleApps/Connection_Manager/bin/connectionmgrppc: application/x-executable; charset=binary
./SampleApps/Gobi_Image_Management/bin/gobiimgmgmtmips: application/x-executable; charset=binary
./SampleApps/Gobi_Image_Management/bin/gobiimgmgmtmipsel: application/x-executable; charset=binary
./SampleApps/Gobi_Image_Management/bin/gobiimgmgmthosti686: application/x-executable; charset=binary
./SampleApps/Gobi_Image_Management/bin/gobiimgmgmtppc: application/x-executable; charset=binary
./SampleApps/Gobi_Image_Management/bin/gobiimgmgmtarm: application/x-executable; charset=binary
./SampleApps/Gobi_Image_Management/bin/gobiimgmgmtarm64: application/x-executable; charset=binary
./SampleApps/Gobi_Image_Management/bin/gobiimgmgmthostx86_64: application/x-executable; charset=binary
./SampleApps/SLQS_Tutorial_Application/bin/slqstutorialarm64: application/x-executable; charset=binary
./SampleApps/SLQS_Tutorial_Application/bin/slqstutorialhostx86_64: application/x-executable; charset=binary
./SampleApps/SLQS_Tutorial_Application/bin/slqstutorialhosti686: application/x-executable; charset=binary
./SampleApps/SLQS_Tutorial_Application/bin/slqstutorialppc: application/x-executable; charset=binary
./SampleApps/SLQS_Tutorial_Application/bin/slqstutorialmipsel: application/x-executable; charset=binary
./SampleApps/SLQS_Tutorial_Application/bin/slqstutorialarm: application/x-executable; charset=binary
./SampleApps/SLQS_Tutorial_Application/bin/slqstutorialmips: application/x-executable; charset=binary
./SampleApps/MC7xxx_Image_Management/bin/mc7xxximgmgmtmipsel: application/x-executable; charset=binary
./SampleApps/MC7xxx_Image_Management/bin/mc7xxximgmgmtppc: application/x-executable; charset=binary
./SampleApps/MC7xxx_Image_Management/bin/mc7xxximgmgmtarm: application/x-executable; charset=binary
./SampleApps/MC7xxx_Image_Management/bin/mc7xxximgmgmtarm64: application/x-executable; charset=binary
./SampleApps/MC7xxx_Image_Management/bin/mc7xxximgmgmtmips: application/x-executable; charset=binary
./SampleApps/MC7xxx_Image_Management/bin/mc7xxximgmgmthostx86_64: application/x-executable; charset=binary
./SampleApps/MC7xxx_Image_Management/bin/mc7xxximgmgmthosti686: application/x-executable; charset=binary
./SampleApps/SMS_Application/bin/SMSSampleApparm64: application/x-executable; charset=binary
./SampleApps/SMS_Application/bin/SMSSampleApphostx86_64: application/x-executable; charset=binary
./SampleApps/SMS_Application/bin/SMSSampleApparm: application/x-executable; charset=binary
./SampleApps/SMS_Application/bin/SMSSampleAppmips: application/x-executable; charset=binary
./SampleApps/SMS_Application/bin/SMSSampleAppmipsel: application/x-executable; charset=binary
./SampleApps/SMS_Application/bin/SMSSampleApphosti686: application/x-executable; charset=binary
./SampleApps/SMS_Application/bin/SMSSampleAppppc: application/x-executable; charset=binary
./SampleApps/CallHandling_Application/bin/callhandlingppc: application/x-executable; charset=binary
./SampleApps/CallHandling_Application/bin/callhandlingarm: application/x-executable; charset=binary
./SampleApps/CallHandling_Application/bin/callhandlingmips: application/x-executable; charset=binary
./SampleApps/CallHandling_Application/bin/callhandlingarm64: application/x-executable; charset=binary
./SampleApps/CallHandling_Application/bin/callhandlinghosti686: application/x-executable; charset=binary
./SampleApps/CallHandling_Application/bin/callhandlingmipsel: application/x-executable; charset=binary
./SampleApps/CallHandling_Application/bin/callhandlinghostx86_64: application/x-executable; charset=binary
./SampleApps/LOC_Service/bin/locservicemipsel: application/x-executable; charset=binary
./SampleApps/LOC_Service/bin/locservicemips: application/x-executable; charset=binary
./SampleApps/LOC_Service/bin/locserviceppc: application/x-executable; charset=binary
./SampleApps/LOC_Service/bin/locservicehosti686: application/x-executable; charset=binary
./SampleApps/LOC_Service/bin/locservicearm64: application/x-executable; charset=binary
./SampleApps/LOC_Service/bin/locservicearm: application/x-executable; charset=binary
./SampleApps/LOC_Service/bin/locservicehostx86_64: application/x-executable; charset=binary
./SampleApps/SWIOMA_Application/bin/SWIOMASampleApparm64: application/x-executable; charset=binary
./SampleApps/SWIOMA_Application/bin/SWIOMASampleApparm: application/x-executable; charset=binary
./SampleApps/SWIOMA_Application/bin/SWIOMASampleApphosti686: application/x-executable; charset=binary
./SampleApps/SWIOMA_Application/bin/SWIOMASampleApphostx86_64: application/x-executable; charset=binary
./SampleApps/SWIOMA_Application/bin/SWIOMASampleAppmips: application/x-executable; charset=binary
./SampleApps/SWIOMA_Application/bin/SWIOMASampleAppmipsel: application/x-executable; charset=binary
./SampleApps/SWIOMA_Application/bin/SWIOMASampleAppppc: application/x-executable; charset=binary
./SampleApps/PDS_Services/bin/pdsservicesarm64: application/x-executable; charset=binary
./SampleApps/PDS_Services/bin/pdsservicesmipsel: application/x-executable; charset=binary
./SampleApps/PDS_Services/bin/pdsservicesmips: application/x-executable; charset=binary
./SampleApps/PDS_Services/bin/pdsserviceshosti686: application/x-executable; charset=binary
./SampleApps/PDS_Services/bin/pdsserviceshostx86_64: application/x-executable; charset=binary
./SampleApps/PDS_Services/bin/pdsservicesarm: application/x-executable; charset=binary
./SampleApps/PDS_Services/bin/pdsservicesppc: application/x-executable; charset=binary
./SampleApps/AirVantageAgent/build.default/runtime/bin/luasignalcmd: application/x-executable; charset=binary
./SampleApps/AirVantageAgent/build.default/runtime/bin/agent: application/x-executable; charset=binary
./SampleApps/AirVantageAgent/build.default/runtime/bin/slqssdk: application/x-executable; charset=binary
./SampleApps/AirVantageAgent/build.default/runtime/bin/appmon_daemon: application/x-executable; charset=binary
./SampleApps/AirVantageAgent/build.default/runtime/bin/slqs_fw_update: application/x-executable; charset=binary
./SampleApps/AirVantageAgent/build.default/CMakeFiles/2.8.12.2/CMakeDetermineCompilerABI_CXX.bin: application/x-executable; charset=binary
./SampleApps/AirVantageAgent/build.default/CMakeFiles/2.8.12.2/CompilerIdCXX/a.out: application/x-executable; charset=binary
./SampleApps/AirVantageAgent/build.default/CMakeFiles/2.8.12.2/CMakeDetermineCompilerABI_C.bin: application/x-executable; charset=binary
./SampleApps/AirVantageAgent/build.default/CMakeFiles/2.8.12.2/CompilerIdC/a.out: application/x-executable; charset=binary
./SampleApps/AirVantageAgent/bin/start_sdkmipsel: application/x-executable; charset=binary
./SampleApps/AirVantageAgent/build.arm/runtime/bin/luasignalcmd: application/x-executable; charset=binary
./SampleApps/AirVantageAgent/build.arm/runtime/bin/agent: application/x-executable; charset=binary
./SampleApps/AirVantageAgent/build.arm/runtime/bin/slqssdk: application/x-executable; charset=binary
./SampleApps/AirVantageAgent/build.arm/runtime/bin/appmon_daemon: application/x-executable; charset=binary
./SampleApps/AirVantageAgent/build.arm/runtime/bin/slqs_fw_update: application/x-executable; charset=binary
./SampleApps/AirVantageAgent/build.arm/CMakeFiles/2.8.12.2/CMakeDetermineCompilerABI_CXX.bin: application/x-executable; charset=binary
./SampleApps/AirVantageAgent/build.arm/CMakeFiles/2.8.12.2/CompilerIdCXX/a.out: application/x-executable; charset=binary
./SampleApps/AirVantageAgent/build.arm/CMakeFiles/2.8.12.2/CMakeDetermineCompilerABI_C.bin: application/x-executable; charset=binary
./SampleApps/AirVantageAgent/build.arm/CMakeFiles/2.8.12.2/CompilerIdC/a.out: application/x-executable; charset=binary
./tools/logging/ramdump-legacy/bin/ramdumptooli386: application/x-executable; charset=binary
./tools/logging/dm/bin/remserialppc: application/x-executable; charset=binary
./tools/logging/dm/bin/remserialarm: application/x-executable; charset=binary
./tools/logging/dm/bin/remserialhosti686: application/x-executable; charset=binary
./tools/logging/dm/bin/remserialmips: application/x-executable; charset=binary
./tools/logging/dm/bin/remserialmipsel: application/x-executable; charset=binary
./tools/logging/dm/bin/split-sqfarm64: application/x-executable; charset=binary
./tools/logging/dm/bin/split-sqfi386: application/x-executable; charset=binary
./tools/logging/dm/bin/split-sqfarm: application/x-executable; charset=binary
./tools/logging/dm/bin/split-sqfhosti686: application/x-executable; charset=binary
./tools/logging/dm/bin/split-sqfppc: application/x-executable; charset=binary
./tools/logging/dm/bin/split-sqfmips: application/x-executable; charset=binary
./tools/logging/dm/bin/remseriali386: application/x-executable; charset=binary
./tools/logging/dm/bin/split-sqfmipsel: application/x-executable; charset=binary
./tools/logging/dm/bin/remserialarm64: application/x-executable; charset=binary
./tools/logging/ramdump/bin/ramdumptool_mips: application/x-executable; charset=binary
./tools/logging/ramdump/bin/ramdumptool_arm: application/x-executable; charset=binary
./tools/logging/ramdump/bin/ramdumptool_hosti686: application/x-executable; charset=binary
./tools/logging/ramdump/bin/ramdumptool_i386: application/x-executable; charset=binary
./tools/logging/ramdump/bin/ramdumptool_arm64: application/x-executable; charset=binary
./tools/logging/ramdump/bin/ramdumptool_ppc: application/x-executable; charset=binary
./tools/logging/ramdump/bin/ramdumptool_mipsel: application/x-executable; charset=binary
./pkgs/qa/qatestppc: application/x-executable; charset=binary
./pkgs/qa/qatestmipsel: application/x-executable; charset=binary
./pkgs/qa/qatestmips: application/x-executable; charset=binary
./pkgs/qa/qatesthostx86_64: application/x-executable; charset=binary
./pkgs/qa/qatesthosti686: application/x-executable; charset=binary
./pkgs/qa/qatestarm64: application/x-executable; charset=binary
./pkgs/qa/qatestarm: application/x-executable; charset=binary
./build/bin/arm/slqssdk: application/x-executable; charset=binary
./build/bin/mipsel/slqssdk: application/x-executable; charset=binary
./build/bin/mips/slqssdk: application/x-executable; charset=binary
./build/bin/arm64/slqssdk: application/x-executable; charset=binary
./build/bin/hosti686/slqssdk: application/x-executable; charset=binary
./build/bin/hostx86_64/slqssdk: application/x-executable; charset=binary
./build/bin/ppc/slqssdk: application/x-executable; charset=binary
```
