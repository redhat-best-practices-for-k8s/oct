#!/bin/bash

DB_FOLDER=cmd/tnf/fetch/data
DUMP_DB_FOLDER=/tmp/dump

rm -rf ${DUMP_DB_FOLDER}/*

if [[ "$OCT_DUMP_ONLY" == "true" ]]; then
    echo "OCT: Dumping current DB to ${DUMP_DB_FOLDER}"
    cp -a ${DB_FOLDER} ${DUMP_DB_FOLDER}
    exit 0
fi

if [[ "${OCT_FORCE_REGENERATE_DB}" == "true" ]]; then
    echo "OCT: Forced db regeneration."
    rm -rf ${DB_FOLDER}/archive.json
    rm -rf ${DB_FOLDER}/containers/*
    rm -rf ${DB_FOLDER}/operators/*
    rm -rf ${DB_FOLDER}/helm/*
fi

# Launch OCT tool to fetch everything.
./oct fetch --operator --container --helm

RESULT=$?
if [ ${RESULT} -ne 0 ]; then 
    echo "ERROR: OCT returned error code."
    exit 1
fi

echo "OCT: Dumping current DB to ${DUMP_DB_FOLDER}"
cp -a ${DB_FOLDER} ${DUMP_DB_FOLDER}
