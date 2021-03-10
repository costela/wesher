#!/usr/bin/env bash

DATE=$(date)
INTERFACE="${1}"
ROUTES=$(ip route show)
WG_ALLOWED_IPS=($(wg show ${1} |grep 'allowed ips' |sed 's/allowed ips://g' |sed 's/^[[:space:]]*//g' |sed 's/,//g'))

for NETWORK in "${WG_ALLOWED_IPS[@]}"
do
	CLEANED_NETWORK=$(echo "${NETWORK}" |sed 's/\/32//g')

	NETWORK_ALREADY_EXISTS=$(echo "${ROUTES}" |grep "${CLEANED_NETWORK}")
	if [ -z "${NETWORK_ALREADY_EXISTS}" ]; then
		echo "${DATE}: network ${NETWORK} is missing, adding route for ${INTERFACE}"
		ip route add ${NETWORK} dev ${INTERFACE}
	fi
done
