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
