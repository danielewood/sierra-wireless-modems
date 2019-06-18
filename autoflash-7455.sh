#!/bin/bash
#.USAGE
# To start, run:
# wget https://raw.githubusercontent.com/danielewood/sierra-wireless-modems/master/autoflash-7455.sh && sudo bash autoflash-7455.sh

#.SYNOPSIS
# - Only for use on Ubuntu 18.04 LTS LiveUSB
# - Changes all models of EM7455/MC7455 Modems to the Generic Sierra Wireless VID/PID
# - Flashes the Current Generic Firmware as of 2018-09-14

#.DESCRIPTION
# - Only for use on Ubuntu 18.04 LTS LiveUSB
# - All Needed Packages will Auto-Install
# - Sets MBIM Mode with AT Commands Access 
# - Changes all models of EM74XX/MC74XX Modems to the Generic Sierra Wireless VID/PID
# - Clears Band Restrictions and Places Modem in LTE only mode.
# - Flashes the Current Generic Firmware

#.NOTES
# License: The Unlicense / CCZero / Public Domain
# Author: Daniel Wood / https://github.com/danielewood

#.LINK
# https://github.com/danielewood/sierra-wireless-modems

#.VERSION
# Version: 20190618

BLUE='\033[0;34m'
NC='\033[0m' # No Color

if [ "$EUID" -ne 0 ]
    then echo "Please run with sudo"
    exit
fi

lsbrelease=`lsb_release -c | awk '{print $2}'`
if [ "$lsbrelease" != "bionic" ]
    then echo "Please run on Ubuntu 18.04 LTS"
    lsb_release -a
    exit
fi

printf "${BLUE}---${NC}\n"
echo 'Searching for EM7455/MC7455 USB modems...'
modemcount=`lsusb | grep -i -E '1199:9071|1199:9079|413C:81B6' | wc -l`
while [ $modemcount -eq 0 ]
do
    printf "${BLUE}---${NC}\n"
    echo "Could not find any EM7455/MC7455 USB modems"
    echo 'Unplug and reinsert the EM7455/MC7455 USB connector...'
    modemcount=`lsusb | grep -i -E '1199:9071|1199:9079|413C:81B6' | wc -l`
    sleep 3
done

printf "${BLUE}---${NC}\n"
echo "Found EM7455/MC7455: 
`lsusb | grep -i -E '1199:9071|1199:9079|413C:81B6'`
"

if [ $modemcount -gt 1 ]
then 
    printf "${BLUE}---${NC}\n"
    echo "Found more than one EM7455/MC7455, remove the one you dont want to flash and try again."
    exit
fi

# Stop modem manager to prevent AT command spam and allow firmware-update
printf "${BLUE}---${NC}\n"
echo 'Stoping modem manager to prevent AT command spam and allow firmware-update, this may take a minute...'
systemctl stop ModemManager
systemctl disable ModemManager

printf "${BLUE}---${NC}\n"
echo "Installing all needed prerequisites..."

sudo add-apt-repository universe
sudo apt update 
# need make and GCC for compiling perl modules
apt-get install make gcc curl minicom libqmi-glib5 libqmi-proxy libqmi-utils -y
# Use cpan to install/compile all dependencies needed by swi_setusbcomp.pl
yes | cpan install UUID::Tiny IPC::Shareable JSON

# Install Modem Mode Switcher
if [ ! -f swi_setusbcomp.pl ]; then
    wget https://git.mork.no/wwan.git/plain/scripts/swi_setusbcomp.pl
    chmod +x ./swi_setusbcomp.pl
fi

devpath=`ls /dev | grep -E 'cdc-wdm|qcqmi'`

# Reset Modem
printf "${BLUE}---${NC}\n"
echo 'Reseting modem...'
./swi_setusbcomp.pl --usbreset --device="/dev/$devpath" &>/dev/null
sleep 3
# Modem Mode Switch to usbcomp=8 (DM   NMEA  AT    MBIM)
printf "${BLUE}---${NC}\n"
echo 'Running Modem Mode Switch to usbcomp=8 (DM   NMEA  AT    MBIM)'
./swi_setusbcomp.pl --usbcomp=8 --device="/dev/$devpath"

# Reset Modem
printf "${BLUE}---${NC}\n"
echo 'Reseting modem...'
./swi_setusbcomp.pl --usbreset --device="/dev/$devpath" &>/dev/null

deviceid=''
while [ -z $deviceid ]
do
    echo 'Waiting for modem to reboot...'
    sleep 3
    deviceid=`lsusb | grep -i -E '1199:9071|1199:9079|413C:81B6' | awk '{print $6}'`
done

sleep 5

ttyUSB=`dmesg | grep '.3: Qualcomm USB modem converter detected' -A1 | grep ttyUSB | sed 's/.*attached\ to\ //' | tail -1`
devpath=`ls /dev | grep -E 'cdc-wdm|qcqmi'`

# cat the serial port to monitor output and commands. cat will exit when AT!RESET kicks off.
sudo cat /dev/$ttyUSB 2>1 | tee -a modem.log &  

# Display current modem settings
printf "${BLUE}---${NC}\n"
echo 'Current modem settings:'
echo 'send AT
send ATE1
sleep 1
send ATI
sleep 1
send AT!ENTERCND=\"A710\"
sleep 1
send AT!IMPREF?
sleep 1
send AT!GOBIIMPREF?
sleep 1
send AT!USBCOMP?
sleep 1
send AT!USBCOMP=?
sleep 1
send AT!USBVID?
sleep 1
send AT!USBPID?
sleep 1
send AT!USBPRODUCT?
sleep 1
send AT!PRIID?
sleep 1
send AT!SELRAT?
sleep 1
send AT!BAND?
sleep 1
send AT!BAND=?
sleep 1
send AT!PCINFO?
sleep 1
send AT!IMAGE?
sleep 1
! pkill minicom
' > script.txt

sudo minicom -b 115200 -D /dev/$ttyUSB -S script.txt &>/dev/null
printf "${BLUE}---${NC}\n"
while [[ ! $REPLY =~ ^[Yy]$ ]]
do
    read -p '
Warning: This will overwrite all settings with generic EM7455/MC7455 Settings.
Are you sure you want to continue? (CTRL+C to exit) ' -n 1 -r
    if [[ $REPLY =~ ^[Nn]$ ]]
    then
        printf '\r\n'; break
    fi
done
printf '\r\n'

# Clear Previous PRI/FW Entries
echo 'send AT!IMAGE=0
sleep 1
send AT!IMAGE?
sleep 1
! pkill minicom
' > script.txt
sudo minicom -b 115200 -D /dev/$ttyUSB -S script.txt &>/dev/null

# Reset Modem
printf "${BLUE}---${NC}\n"
echo 'Reseting modem...'
./swi_setusbcomp.pl --usbreset --device="/dev/$devpath" &>/dev/null

# Find latest 7455 firmware and download it
SWI9X30C_ZIP=`curl https://source.sierrawireless.com/resources/airprime/minicard/74xx/airprime-em_mc74xx-approved-fw-packages/ 2> /dev/null | grep PTCRB -B1 | grep -iEo '7455/swi9x30c[_0-9.]+_generic_[_0-9.]+' | cut -c 6- | tail -n1`
SWI9X30C_ZIP="$SWI9X30C_ZIP"'zip'

SWI9X30C_URL='https://source.sierrawireless.com/~/media/support_downloads/airprime/74xx/fw/7455/'"$SWI9X30C_ZIP"

SWI9X30C_LENGTH=`curl -sI $SWI9X30C_URL | grep -i Content-Length | grep -Eo '[0-9]+'`

# If remote file size is less than 40MiB, something went wrong, exit.
if [[ $SWI9X30C_LENGTH -lt 40000000 ]]; then
    printf "${BLUE}---${NC}\n"
    printf "Download of ${BLUE}$SWI9X30C_ZIP${NC} failed.\nFile size on server is too small, something is wrong, exiting...\n"
    printf "Attempted download URL was: $SWI9X30C_URL\n"
    printf "curl info:\n"
    curl -sI $SWI9X30C_URL
    printf "${BLUE}---${NC}\n"
    exit
fi

echo "Downloading $SWI9X30C_URL"
curl -O $SWI9X30C_URL

# If download size does not match what server says, exit:
if [ $SWI9X30C_LENGTH -ne `stat --printf="%s" $SWI9X30C_ZIP` ]; then
    printf "${BLUE}---${NC}\n"
    printf "Download of ${BLUE}$SWI9X30C_ZIP${NC} failed.\nDownloaded file size is inconsistent with server, exiting...\n"
    printf "${BLUE}---${NC}\n"
    exit
fi

# Unzip SWI9X30C, force overwrite
unzip -o "$SWI9X30C_ZIP"

SWI9X30C_CWE=`find -maxdepth 1 -type f -iregex '.*SWI9X30C[0-9_.]+\.cwe' | cut -c 3- | tail -n1`
SWI9X30C_NVU=`find -maxdepth 1 -type f -iregex '.*SWI9X30C[0-9_.]+generic[0-9_.]+\.nvu' | cut -c 3- | tail -n1`

deviceid=`lsusb | grep -i -E '1199:9071|1199:9079|413C:81B6' | awk '{print $6}'`
while [ -z $deviceid ]
do
    echo 'Waiting for modem to reboot...'
    sleep 3
    deviceid=`lsusb | grep -i -E '1199:9071|1199:9079|413C:81B6' | awk '{print $6}'`
done
#Kill cat processes used for monitoring status, if it hasnt already exited
sudo pkill -9 cat &>/dev/null

printf "${BLUE}---${NC}\n"
echo "Flashing $SWI9X30C_CWE onto Generic Sierra Modem..."
sleep 5
qmi-firmware-update --update -d "$deviceid" "$SWI9X30C_CWE" "$SWI9X30C_NVU"
rc=$?
if [[ $rc != 0 ]]
then
    echo "Firmware Update failed, exiting..."
    exit $rc
fi

deviceid=''
while [ -z $deviceid ]
do
    echo 'Waiting for modem to reboot...'
    sleep 3
    deviceid=`lsusb | grep -i -E '1199:9071|1199:9079|413C:81B6' | awk '{print $6}'`
done

# cat the serial port to monitor output and commands. cat will exit when AT!RESET kicks off.
sudo cat /dev/$ttyUSB 2>1 | tee -a modem.log &  

# Set Generic Sierra Wireless VIDs/PIDs
if [[ $REPLY =~ ^[Yy]$ ]]
then
    echo 'send AT
send ATE1
sleep 1
send ATI
sleep 1
send AT!ENTERCND=\"A710\"
sleep 1
send AT!IMPREF=\"GENERIC\"
sleep 1
send AT!GOBIIMPREF=\"GENERIC\"
sleep 1
send AT!USBCOMP=1,1,0000100D
sleep 1
send AT!USBVID=1199
sleep 1
send AT!USBPID=9071,9070
sleep 1
send AT!USBPRODUCT=\"EM7455\"
sleep 1
send AT!PRIID=\"9904609\",\"002.030\",\"Generic-Laptop\"
sleep 1
send AT!SELRAT=06
sleep 1
send AT!BAND=00
sleep 1
send AT!IMAGE?
sleep 1
send AT!PCINFO?
sleep 1
send AT!RESET
! pkill minicom
' > script.txt
    sudo minicom -b 115200 -D /dev/$ttyUSB -S script.txt &>/dev/null
fi

#Done, restart ModemManager
systemctl enable ModemManager &>/dev/null
systemctl start ModemManager &>/dev/null

echo "Done!"

#Kill cat processes used for monitoring status, if it hasnt already exited
sudo pkill -9 cat &>/dev/null
