#!/bin/bash
# shellcheck disable=SC2059
#
#.USAGE
# To start, run:
# wget https://raw.githubusercontent.com/danielewood/sierra-wireless-modems/master/autoflash-7455.sh && sudo bash autoflash-7455.sh

#.SYNOPSIS
# - Only for use on Ubuntu 18 (Bionic) LiveUSB
# - Changes all models of EM7455/MC7455 Modems to the Generic Sierra Wireless VID/PID
# - Flashes the Current Generic Firmware

#.DESCRIPTION
# - Only for use on Ubuntu 18 (Bionic) LiveUSB
# - All Needed Packages will Auto-Install
# - Sets MBIM Mode with AT Commands Access
# - Changes all models of EM74XX/MC74XX Modems to the Generic Sierra Wireless VID/PID
# - Clears Band Restrictions and Places Modem in LTE only mode.
# - Flashes the Current Generic Firmware
# - Sets PCOFFEN=2 to tell the modem to ignore the W_DISABLE pin sent by many laptop's internal M2 slots.
# - Sets FASTENUMEN=2 to skip bootloader on warm-boots.
#   - This, combined with PCOFFEN enables these modems to work in the X1G6/T470 and newer laptops.

#.NOTES
# License: The Unlicense / CCZero / Public Domain
# Author: Daniel Wood / https://github.com/danielewood

#.LINK
# https://github.com/danielewood/sierra-wireless-modems

#.VERSION
# Version: 20190724

##################
### Pre-Checks ###
##################

if [ "$EUID" -ne 0 ]
    then echo "Please run with sudo"
    exit
fi

lsbrelease=$(lsb_release -c | awk '{print $2}')
if [ "$lsbrelease" != "bionic" ]
    then echo "Please run on Ubuntu 18 (Bionic)"
    lsb_release -a
    exit
fi

#########################
### Variables & Input ###
#########################

CYAN='\033[0;36m'
NC='\033[0m' # No Color

function display_usage() {
  printf "${CYAN}Usage:${NC} $0\n"
  printf "\n"
  printf "${CYAN}Modem Modes:${NC}\n"
  printf " -h    Display usage instructions\n"
  printf " -u    Set AT!USBSPEED\n"
  printf "         0 - High Speed USB 2 ${CYAN}[Default]${NC}\n"
  printf "         1 - SuperSpeed USB 3\n"
  printf " -m    Set AT!USBCOMP\n"
  printf "         8 - MBIM mode (diag,nmea,modem,mbim) ${CYAN}[Default]${NC}\n"
  printf "         6 - QMI mode (diag,nmea,modem,rmnet0)\n"
  printf " -b    Set AT!SELRAT and AT!BAND\n"
  printf "         9 - LTE Only ${CYAN}[Default]${NC}\n"
  printf "         0 - All Bands\n"  
  printf " -e    Set AT!FASTENUMEN\n"
  printf "         0 - Disable fast enumeration\n"
  printf "         1 - Enable fast enumeration for cold boot and disable for warm boot\n"
  printf "         2 - Enable fast enumeration for warm boot and disable for cold boot ${CYAN}[Default]${NC}\n"
  printf "         3 - Enable fast enumeration for warm and cold boot\n"
  printf "\n"
  printf "${CYAN}Script Modes:${NC}\n"
  printf " -a    Enable All Script Functions ${CYAN}[Default]${NC}\n"
  printf "         Same as: $0 -Mgcdfs\n"
  printf " -M    Use swi_setusbcomp.pl to set modem composition\n"
  printf " -g    Display Modem Settings\n"
  printf " -c    Clear Existing Modem Firmwares\n"
  printf " -d    Download and Unpack Modem Firmware from Sierra Wireless\n"
  printf " -f    Flash Modem Firmware\n"
  printf " -s    Change Modem Settings/Modes\n"
  printf " -l    Legacy Stable Firmware\n"
  printf "         Set SWI9X30C_ZIP=\"SWI9X30C_02.30.01.01_GENERIC_002.045_001.zip\"\n"
  printf " -q    Quiet Mode -- suppress most output\n"
  printf " -v    Verbose/Debug Mode\n"
  printf "\n"
  exit 0
}

while getopts hu:m:b:e:Mgcdfsalqv option
do
 case "${option}"
 in
 h) display_usage
      exit 0;;
 u) AT_USBSPEED=${OPTARG};;
 m) AT_USBCOMP=${OPTARG};;
 b) AT_SELRAT=${OPTARG};;
 e) AT_FASTENUMEN=${OPTARG};;
 M) set_swi_setusbcomp_trigger=1;;
 g) get_modem_settings_trigger=1;;
 c) clear_modem_firmware_trigger=1;;
 d) download_modem_firmware_trigger=1;;
 f) flash_modem_firmware_trigger=1;;
 s) set_modem_settings_trigger=1;;
 a) all_functions_trigger=1;;
 l) SWI9X30C_ZIP='SWI9X30C_02.30.01.01_GENERIC_002.045_001.zip';;
 q) quiet_trigger=1;;
 v) verbose_trigger=1;;
 *) display_usage>&2
      exit 1;;
  esac
done


#################
### Functions ###
#################

# If no options are set, use defaults of -Mgcdfs
if [[ -z $get_modem_settings_trigger && -z $clear_modem_firmware_trigger \
    && -z $download_modem_firmware_trigger && -z $flash_modem_firmware_trigger \
    && -z $set_modem_settings_trigger && -z $set_swi_setusbcomp_trigger ]]; then
        all_functions_trigger=1
fi

if [[ all_functions_trigger -eq 1 ]]; then
  get_modem_settings_trigger=1
  clear_modem_firmware_trigger=1
  download_modem_firmware_trigger=1
  flash_modem_firmware_trigger=1
  set_modem_settings_trigger=1
  set_swi_setusbcomp_trigger=1
fi

function display_variables() {
    echo "AT_USBSPEED=$AT_USBSPEED"
    echo "AT_USBCOMP=$AT_USBCOMP"
    echo "AT_SELRAT=$AT_SELRAT"
    echo "AT_FASTENUMEN=$AT_FASTENUMEN"
    echo "all_functions_trigger=$all_functions_trigger"
    echo "get_modem_settings_trigger=$get_modem_settings_trigger"
    echo "clear_modem_firmware_trigger=$clear_modem_firmware_trigger"
    echo "download_modem_firmware_trigger=$download_modem_firmware_trigger"
    echo "flash_modem_firmware_trigger=$flash_modem_firmware_trigger"
    echo "set_modem_settings_trigger=$set_modem_settings_trigger"
    echo "set_swi_setusbcomp_trigger=$set_swi_setusbcomp_trigger"
    echo "SWI9X30C_CWE=$SWI9X30C_CWE"
    echo "SWI9X30C_NVU=$SWI9X30C_NVU"
    echo "AT_PRIID_STRING=$AT_PRIID_STRING"
    echo "AT_PRIID_PN=$AT_PRIID_PN"
    echo "AT_PRIID_REV=$AT_PRIID_REV"
}

function set_options() {
    # See if QMI desired, otherwise default to MBIM
    if [[ ${AT_USBCOMP^^} =~ ^QMI$|^6$ ]]; then
        echo 'Setting QMI Mode for Modem'
        echo 'Interface bitmask: 0000010D (diag,nmea,modem,rmnet0)'
        AT_USBCOMP="1,1,0000010D"
        swi_usbcomp='6'
    else
        echo 'Setting MBIM Mode for Modem'
        echo 'Interface bitmask: 0000100D (diag,nmea,modem,mbim)'
        AT_USBCOMP="1,1,0000100D"
        swi_usbcomp='8'
    fi

    # Check for ALL/00 bands and set correct SELRAT/BAND, otherwise default to LTE
    if [[ ${AT_SELRAT^^} =~ ^ALL$|^0$|^00$ ]]; then
        AT_SELRAT='00'
        AT_BAND='00'
    else
        AT_SELRAT='06'
        AT_BAND='09'
    fi

    # Check if valid FASTENUMEN mode, otherwise default to 2
    if [[ ! $AT_FASTENUMEN =~ ^[0-3]$ ]]; then
        AT_FASTENUMEN=2
    fi

    #FASTENUMEN_MODES="0 = Disable fast enumeration [Default]
    #1 = Enable fast enumeration for cold boot and disable for warm boot
    #2 = Enable fast enumeration for warm boot and disable for cold boot
    #3 = Enable fast enumeration for warm and cold boot"
    #echo '"FASTENUMEN"—Enable/disable fast enumeration for warm/cold boot.'
    #echo -n 'Set mode: ' && echo "$FASTENUMEN_MODES" | grep -E "^$AT_FASTENUMEN"

    # Check desired USB interface mode, otherwise default to 0 (USB 2.0)
    if [[ ${AT_USBSPEED^^} =~ SUPER|USB3|1 ]]; then
        AT_USBSPEED=1
    else
        AT_USBSPEED=0
    fi
}

function get_modem_deviceid() {
    deviceid=''
    while [ -z $deviceid ]
    do
        echo 'Waiting for modem to reboot...'
        sleep 3
        deviceid=$(lsusb | grep -i -E '1199:9071|1199:9079|413C:81B6' | awk '{print $6}')
    done
    sleep 3
    ttyUSB=$(dmesg | grep '.3: Qualcomm USB modem converter detected' -A1 | grep -Eo 'ttyUSB[0-9]$' | tail -1)
    devpath=$(find /dev -maxdepth 1 -regex '/dev/cdc-wdm[0-9]' -o -regex '/dev/qcqmi[0-9]')
}

function reset_modem {
    # Reset Modem
    printf "${CYAN}---${NC}\n"
    echo 'Reseting modem...'
    ./swi_setusbcomp.pl --usbreset --device="$devpath" &>/dev/null
}

function get_modem_settings() {
    # cat the serial port to monitor output and commands. cat will exit when AT!RESET kicks off.
    sudo cat /dev/"$ttyUSB" 2>&1 | tee -a modem.log &  

    # Display current modem settings
    printf "${CYAN}---${NC}\n"
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
send AT!USBSPEED?
sleep 1
send AT!USBSPEED=?
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
send AT!PCOFFEN?
sleep 1
send AT!CUSTOM?
sleep 1
send AT!IMAGE?
sleep 1
! pkill cat
sleep 1
! pkill minicom
' > script.txt
    sudo minicom -b 115200 -D /dev/"$ttyUSB" -S script.txt &>/dev/null
}

function clear_modem_firmware() {
    # cat the serial port to monitor output and commands. cat will exit when AT!RESET kicks off.
    sudo cat /dev/"$ttyUSB" 2>&1 | tee -a modem.log &  
    # Clear Previous PRI/FW Entries
    echo 'send AT
send AT!IMAGE=0
sleep 1
send AT!IMAGE?
sleep 1
! pkill cat
sleep 1
! pkill minicom
' > script.txt
    sudo minicom -b 115200 -D /dev/"$ttyUSB" -S script.txt &>/dev/null
}

function download_modem_firmware() {
    # Find latest 7455 firmware and download it
    if [[ -z $SWI9X30C_ZIP ]]; then
        SWI9X30C_ZIP=$(curl https://source.sierrawireless.com/resources/airprime/minicard/74xx/airprime-em_mc74xx-approved-fw-packages/ 2> /dev/null | grep PTCRB -B1 | grep -iEo '7455/swi9x30c[_0-9.]+_generic_[_0-9.]+' | cut -c 6- | tail -n1)
        SWI9X30C_ZIP="${SWI9X30C_ZIP^^}"'zip'
    fi
    SWI9X30C_URL='https://source.sierrawireless.com/~/media/support_downloads/airprime/74xx/fw/7455/'"$SWI9X30C_ZIP"

    SWI9X30C_LENGTH=$(curl -sI "$SWI9X30C_URL" | grep -i Content-Length | grep -Eo '[0-9]+')

    # If remote file size is less than 40MiB, something went wrong, exit.
    if [[ $SWI9X30C_LENGTH -lt 40000000 ]]; then
        printf "${CYAN}---${NC}\n"
        printf "Download of ${CYAN}$SWI9X30C_ZIP${NC} failed.\nFile size on server is too small, something is wrong, exiting...\n"
        printf "Attempted download URL was: $SWI9X30C_URL\n"
        printf "curl info:\n"
        curl -sI "$SWI9X30C_URL"
        printf "${CYAN}---${NC}\n"
        exit
    fi

    if [[ $SWI9X30C_LENGTH -eq $(stat --printf="%s" "$SWI9X30C_ZIP" 2>/dev/null) ]]; then
        echo "Already downloaded $SWI9X30C_ZIP..."
    else
        echo "Downloading $SWI9X30C_URL"
        curl -o "$SWI9X30C_ZIP" "$SWI9X30C_URL"
    fi

    # If download size does not match what server says, exit:
    if [[ $SWI9X30C_LENGTH -ne $(stat --printf="%s" "$SWI9X30C_ZIP" 2>/dev/null) ]]; then
        printf "${CYAN}---${NC}\n"
        printf "Download of ${CYAN}$SWI9X30C_ZIP${NC} failed.\nDownloaded file size is inconsistent with server, exiting...\n"
        printf "${CYAN}---${NC}\n"
        exit
    fi

    # Cleanup old CWE/NVUs
    rm -f ./*.cwe ./*.nvu 2>/dev/null
    
    # Unzip SWI9X30C, force overwrite
    unzip -o "$SWI9X30C_ZIP"
}

function flash_modem_firmware() {
    # Kill cat processes used for monitoring status, if it hasnt already exited
    sudo pkill -9 cat &>/dev/null

    printf "${CYAN}---${NC}\n"
    echo "Flashing $SWI9X30C_CWE onto Generic Sierra Modem..."
    sleep 5
    qmi-firmware-update --update -d "$deviceid" "$SWI9X30C_CWE" "$SWI9X30C_NVU"
    rc=$?
    if [[ $rc != 0 ]]
    then
        echo "Firmware Update failed, exiting..."
        exit $rc
    fi
}

function set_modem_settings() {
    # cat the serial port to monitor output and commands. cat will exit when AT!RESET kicks off.
    sudo cat /dev/"$ttyUSB" 2>&1 | tee -a modem.log &  

    # Set Generic Sierra Wireless VIDs/PIDs
    cat <<EOF > script.txt
send AT
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
send AT!USBCOMP=$AT_USBCOMP
sleep 1
send AT!USBVID=1199
sleep 1
send AT!USBPID=9071,9070
sleep 1
send AT!USBPRODUCT=\"EM7455\"
sleep 1
send AT!PRIID=\"$AT_PRIID_PN\",\"$AT_PRIID_REV\",\"Generic-Laptop\"
sleep 1
send AT!SELRAT=$AT_SELRAT
sleep 1
send AT!BAND=$AT_BAND
sleep 1
send AT!CUSTOM=\"FASTENUMEN\",$AT_FASTENUMEN
sleep 1
send AT!PCOFFEN=2
sleep 1
send AT!PCOFFEN?
sleep 1
send AT!USBSPEED=$AT_USBSPEED
sleep 1
send AT!USBSPEED?
sleep 1
send AT!USBSPEED=?
sleep 1
send AT!CUSTOM?
sleep 1
send AT!IMAGE?
sleep 1
send AT!PCINFO?
sleep 1
send AT!RESET
! pkill minicom
EOF
    sudo minicom -b 115200 -D /dev/"$ttyUSB" -S script.txt &>/dev/null
}

function script_prechecks() {
    printf "${CYAN}---${NC}\n"
    echo 'Searching for EM7455/MC7455 USB modems...'
    modemcount=$(lsusb | grep -c -i -E '1199:9071|1199:9079|413C:81B6')
    while [ "$modemcount" -eq 0 ]
    do
        printf "${CYAN}---${NC}\n"
        echo "Could not find any EM7455/MC7455 USB modems"
        echo 'Unplug and reinsert the EM7455/MC7455 USB connector...'
        modemcount=$(lsusb | grep -c -i -E '1199:9071|1199:9079|413C:81B6')
        sleep 3
    done

    printf "${CYAN}---${NC}\n"
    echo "Found EM7455/MC7455: 
    $(lsusb | grep -i -E '1199:9071|1199:9079|413C:81B6')
    "

    if [ "$modemcount" -gt 1 ]
    then 
        printf "${CYAN}---${NC}\n"
        echo "Found more than one EM7455/MC7455, remove the one you dont want to flash and try again."
        exit
    fi

    # Stop modem manager to prevent AT command spam and allow firmware-update
    printf "${CYAN}---${NC}\n"
    echo 'Stoping modem manager to prevent AT command spam and allow firmware-update, this may take a minute...'
    systemctl stop ModemManager
    systemctl disable ModemManager

    printf "${CYAN}---${NC}\n"
    echo "Installing all needed prerequisites..."
    add-apt-repository universe 1>/dev/null
    apt update -y
    # need make and GCC for compiling perl modules
    apt-get install make gcc curl minicom libqmi-glib5 libqmi-proxy libqmi-utils -y
    # Use cpan to install/compile all dependencies needed by swi_setusbcomp.pl
    yes | cpan install UUID::Tiny IPC::Shareable JSON

    # Install Modem Mode Switcher
    if [ ! -f swi_setusbcomp.pl ]; then
        wget https://git.mork.no/wwan.git/plain/scripts/swi_setusbcomp.pl
    fi
    chmod +x ./swi_setusbcomp.pl

    reset_modem
}

function set_swi_setusbcomp() {
    # Modem Mode Switch to usbcomp=8 (DM   NMEA  AT    MBIM)
    printf "${CYAN}---${NC}\n"
    echo "Running Modem Mode Switch to usbcomp=$swi_usbcomp"
    ./swi_setusbcomp.pl --usbcomp=$swi_usbcomp --device="$devpath"
    reset_modem
}

function script_cleanup() {
    # Restart ModemManager
    systemctl enable ModemManager &>/dev/null
    systemctl start ModemManager &>/dev/null

    echo "Done!"

    # Kill cat processes used for monitoring status, if it hasnt already exited
    sudo pkill -9 cat &>/dev/null
}

########################
### Script Execution ###
########################

if [[ $quiet_trigger ]]; then
    script_prechecks &>/dev/null
else
    script_prechecks
fi

set_options

devpath=$(find /dev -maxdepth 1 -regex '/dev/cdc-wdm[0-9]' -o -regex '/dev/qcqmi[0-9]')

if [[ $set_swi_setusbcomp_trigger ]]; then
    set_swi_setusbcomp
fi

get_modem_deviceid

[[ $get_modem_settings_trigger ]] && get_modem_settings

if [[ $clear_modem_firmware_trigger ]]; then
  clear_modem_firmware
  get_modem_deviceid
fi

[[ $download_modem_firmware_trigger ]] && download_modem_firmware

SWI9X30C_CWE=$(find . -maxdepth 1 -type f -iregex '.*SWI9X30C[0-9_.]+\.cwe' | cut -c 3- | tail -n1)
SWI9X30C_NVU=$(find . -maxdepth 1 -type f -iregex '.*SWI9X30C[0-9_.]+generic[0-9_.]+\.nvu' | cut -c 3- | tail -n1)

AT_PRIID_STRING=$(strings "$SWI9X30C_NVU" | grep '^9999999_.*_SWI9X30C_' | sort -u | head -1)
AT_PRIID_PN="$(echo "$AT_PRIID_STRING" | awk -F'_' '{print $2}')"
AT_PRIID_REV="$(echo "$AT_PRIID_STRING" | grep -Eo '[0-9]{3}\.[0-9]{3}')"

[[ $flash_modem_firmware_trigger ]] && flash_modem_firmware
[[ $set_modem_settings_trigger ]] && set_modem_settings
[[ $verbose_trigger ]] && display_variables

script_cleanup

# Done