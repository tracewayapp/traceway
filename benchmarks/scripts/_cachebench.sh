#!/usr/bin/env bash
# Shared matrix helpers for the symbolicator cache benchmark drivers. Source
# this after setting ENTRIES/COLD_RATIOS/UNBOUNDED and defining a
# run_cell <label> <mode> <entries> <ratio> function for the transport
# (local exec in cachebench-local.sh, ssh in cachebench-entry.sh).

parse_matrix() {
    IFS=',' read -ra entries_list <<< "${ENTRIES}"
    IFS=',' read -ra ratios_list <<< "${COLD_RATIOS}"
    max_entries=0
    local n
    for n in "${entries_list[@]}"; do
        (( n > max_entries )) && max_entries=$n
    done
}

write_stub() {
    local out="$1" tier="$2" label="$3" mode="$4" n="$5" hot="$6" ratio="$7" rc="$8"
    local status="error"
    case "${rc}" in
        137) status="oom" ;;
        124) status="timeout" ;;
    esac
    printf '{"status":"%s","tier":"%s","label":"%s","mode":"%s","entries":%s,"hot":%s,"coldRatio":%s,"exitCode":%s}\n' \
        "${status}" "${tier}" "${label}" "${mode}" "${n}" "${hot}" "${ratio}" "${rc}" > "${out}"
}

run_sweep() {
    local ratio n
    for ratio in "${ratios_list[@]}"; do
        for n in "${entries_list[@]}"; do
            run_cell memory memory "${n}" "${ratio}"
            run_cell disk disk "${n}" "${ratio}"
            if [[ "${UNBOUNDED}" == "1" && "${ratio}" != "0" ]]; then
                run_cell memory-unbounded memory "${n}" "${ratio}"
            fi
        done
    done
}
