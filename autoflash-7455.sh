#/bin/bash
#.USAGE
# To start, run:
# wget https://raw.githubusercontent.com/danielewood/sierra-wireless-modems/master/autoflash-7455.sh && sudo bash autoflash-7455.sh

#.SYNOPSIS
# - Only for use on Ubuntu 18.04 LTS LiveUSB
# - Changes all models of EM7455/MC7455 Modems to the Generic Sierra Wireless VID/PID
# - Flashes the Current Generic Firmware as of 2018-07-18

#.DESCRIPTION
# - Only for use on Ubuntu 18.04 LTS LiveUSB
# - All Needed Packages will Auto-Install
# - Sets MBIM Mode with AT Commands Access 
# - Changes all models of EM7455/MC7455 Modems to the Generic Sierra Wireless VID/PID
# - Clears Band Restrictions and Places Modem in LTE only mode.
# - Flashes the Current Generic Firmware as of 2018-07-18

#.NOTES
# License: The Unlicense / CCZero / Public Domain
# Author: Daniel Wood / https://github.com/danielewood

#.LINK
# https://github.com/danielewood/sierra-wireless-modems

#.VERSION
# Version: 20180719

BLUE='\033[0;34m'
NC='\033[0m' # No Color

if [ "$EUID" -ne 0 ]
    then echo "Please run with sudo or as root"
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
modemcount=`lsusb | grep -E '1199:9071|1199:9079|413C:81B6' | wc -l`
while [ $modemcount -eq 0 ]
do
    printf "${BLUE}---${NC}\n"
    echo "Could not find any EM7455/MC7455 USB modems"
    echo 'Unplug and reinsert the EM7455/MC7455 USB connector...'
    modemcount=`lsusb | grep -E '1199:9071|1199:9079|413C:81B6' | wc -l`
    sleep 5
done

printf "${BLUE}---${NC}\n"
echo "Found EM7455/MC7455: 
`lsusb | grep -E '1199:9071|1199:9079|413C:81B6'`
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
apt-get update
# need make and GCC for compiling perl modules
apt-get install make gcc curl -y
# Use cpan to install/compile all dependencies needed by swi_setusbcomp.pl
yes | cpan install UUID::Tiny IPC::Shareable JSON

# apt-get will fail to download minicom/qmi-utilities on LiveCD/LiveUSB without adding repositories
# Also, if you add security.ubuntu.com bionic main universe, you'll get an older version of libqmi (1.18)
# So we'll pull the .deb files directly
wget http://security.ubuntu.com/ubuntu/pool/universe/m/minicom/minicom_2.7.1-1_amd64.deb
dpkg -i minicom_2.7.1-1_amd64.deb
wget http://security.ubuntu.com/ubuntu/pool/main/libq/libqmi/libqmi-glib5_1.20.0-1ubuntu1_amd64.deb
dpkg -i libqmi-glib5_1.20.0-1ubuntu1_amd64.deb
wget http://security.ubuntu.com/ubuntu/pool/main/libq/libqmi/libqmi-proxy_1.20.0-1ubuntu1_amd64.deb
dpkg -i libqmi-proxy_1.20.0-1ubuntu1_amd64.deb
wget http://security.ubuntu.com/ubuntu/pool/universe/libq/libqmi/libqmi-utils_1.20.0-1ubuntu1_amd64.deb
dpkg -i libqmi-utils_1.20.0-1ubuntu1_amd64.deb

# Install Modem Mode Switcher
wget https://git.mork.no/wwan.git/plain/scripts/swi_setusbcomp.pl
chmod +x ~/swi_setusbcomp.pl

# Reset modem's USB connection before making changes
printf "${BLUE}---${NC}\n"
echo 'Reseting modem\'s USB connection before making changes'
~/swi_setusbcomp.pl --usbreset &>/dev/null

# Modem Mode Switch to usbcomp=8 (DM   NMEA  AT    MBIM)
printf "${BLUE}---${NC}\n"
echo "Running Modem Mode Switch to usbcomp=8 (DM   NMEA  AT    MBIM)'
~/swi_setusbcomp.pl --usbcomp=8

# Reset Modem
printf "${BLUE}---${NC}\n"
echo 'Reseting modem...'
~/swi_setusbcomp.pl --usbreset &>/dev/null

ttyUSB=`dmesg | tail | grep '.3: Qualcomm USB modem converter detected' -A1 | grep ttyUSB | awk '{print $12}' | sort -u`

# cat the serial port to monitor output and commands. cat will exit when AT!RESET kicks off.
sudo cat /dev/$ttyUSB &  

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

# Set Generic Sierra Wireless VIDs/PIDs
if [[ $REPLY =~ ^[Yy]$ ]]
then
    echo 'send AT
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
send AT!PRIID=\"9904609\",\"002.026\",\"Generic-Laptop\"
sleep 1
send AT!SELRAT=06
sleep 1
send AT!BAND=00
sleep 1
send AT!IMAGE=0
sleep 1
send AT!RESET
! pkill minicom
' > script.txt
    sudo minicom -b 115200 -D /dev/$ttyUSB -S script.txt &>/dev/null
fi

printf "${BLUE}---${NC}\n"
echo 'Download and unzip SWI9X30C_02.24.05.06_GENERIC_002.026_000 firmware'
curl -o SWI9X30C_02.24.05.06_Generic_002.026_000.zip -L https://source.sierrawireless.com/~/media/support_downloads/airprime/74xx/fw/02_24_05_06/7430/swi9x30c_02.24.05.06_generic_002.026_000.ashx 
unzip SWI9X30C_02.24.05.06_Generic_002.026_000.zip

#Kill cat processes used for monitoring status, if it hasnt already exited
sudo pkill -9 cat &>/dev/null

# Reset Modem
printf "${BLUE}---${NC}\n"
echo 'Reseting modem...'
~/swi_setusbcomp.pl --usbreset &>/dev/null

printf "${BLUE}---${NC}\n"
# Flash SWI9X30C_02.24.05.06_GENERIC_002.026_000 onto Generic Sierra Modem
echo 'Flashing SWI9X30C_02.24.05.06_GENERIC_002.026_000 onto Generic Sierra Modem...'
qmi-firmware-update --update -d "1199:9071" SWI9X30C_02.24.05.06.cwe SWI9X30C_02.24.05.06_GENERIC_002.026_000.nvu

#Done, restart ModemManager
systemctl enable ModemManager &>/dev/null
systemctl start ModemManager &>/dev/null

echo "Done!"
