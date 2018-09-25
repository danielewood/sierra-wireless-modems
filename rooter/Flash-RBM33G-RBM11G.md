### Installing OpenWRT/ROOter on a RBM33G/RBM11G
#### 0. WARNING: [Backup your MikroTik License key in case you ever want to go back to RouterOS.](https://openwrt.org/toh/mikrotik/common#saving_mikrotik_routerboard_software_key)

1. Download the latest
    + ROOter for the RBM33G
    + [Putty portable](https://the.earth.li/~sgtatham/putty/latest/w64/putty.exe)
    + [tftpd64 portable](http://www.tftpd64.com/tftpd32_download.html)

1. Extract all of the above to: `C:\Users\Public\Downloads`

1. Setup Network Interface connected to RBM33G/RMB11G POE port:
    + IP: 192.168.1.2
    + Mask: 255.255.255.0
    + Gateway: 192.168.1.1
    + DNS1: 192.168.1.1

    ![](https://i.imgur.com/BWhmjl5.png)
    
1. Disable Windows Firewall
    + From an administrator command prompt:
    + `NetSh Advfirewall set allprofiles state off`
    
    ![](https://i.imgur.com/0jXzvOm.png)

1. Configure and start tftpd32/64
    + Option 1: Overwrite tftpd32.ini with [this one](tftpd32.ini) and start tftpd64.
    + Change the BootFile name if needed.
    
    ![](https://i.imgur.com/03dXtO0.png)
    + Option 2: Use screenshots for the settings, restart tftpd64 after you make your changes 

    ![](https://i.imgur.com/pnYstop.png)![](https://i.imgur.com/Xz2mJpk.png)


1. If you do not have a serial cable (Null Modem DB9 for the RBM33G, TX/RX/GND TTL for the RBM11G):
    - Plug in/powercycle your router, holding the reset button for 20 seconds. This will booth the router in Network Boot mode.
    - After 60 seconds, proceed to step 11, also skip step 15.

1. Open Putty to the COM port of your USB to Serial Adapter with a baud rate of 115200
![](https://i.imgur.com/iwJeDS4.png)

1. Plug in/powercycle your router, press any key within 2 seconds to enter setup. 
![](https://i.imgur.com/m1SRMRw.png)

1. Boot from the network by pressing the following key sequence:
    + o - boot options
    + e - ethernet
    + o - boot options
    + b - boot

    ![](https://i.imgur.com/FX7KJBA.png)

1. Wait for boot to complete (~45 seconds)
![](https://i.imgur.com/J0384lc.png)

1. Go to http://192.168.1.1/cgi-bin/luci/admin/system/flashops and sign in
![](https://i.imgur.com/KJMCUCg.png)

1. Browse to `openwrt-RBM33G-GO2018-08-15-upgrade.bin`
![](https://i.imgur.com/79P3OwT.png)

1. Click Flash Image, continue.
![](https://i.imgur.com/wnGpHNw.png)

1. Re-Enable Windows Firewall
    + From an administrator command prompt:
    + `NetSh Advfirewall set allprofiles state on`

    ![](https://i.imgur.com/4Esgxcs.png)
    
1. Wait for router to finish rebooting, re-enter setup, and reset to default boot options by pressing `r`.
![](https://i.imgur.com/3cKGCde.png)

1. Done. Reset your Windows Network IP settings.
