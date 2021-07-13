export default {
  header: { support: "Support" },
  footer: {
    version: {
      version: "version",
      availableShort: "available",
      availableLong: "update available",
      modalTitle: "Update available",
      modalUpdateStarted: "Evcc will restart after the update..",
      modalInstalledVersion: "Currently installed version",
      modalNoReleaseNotes:
        "No release notes available. More information about the new version can be found here:",
      modalCancel: "Cancel",
      modalUpdate: "Update",
      modalUpdateNow: "Update now",
      modalDownload: "Download",
      modalUpdateStatusStart: "Update started: ",
      modalUpdateStatusFailed: "Update failed: ",
    },
    sponsor: {
      sponsoredShort: "thanks",
      sponsoredLong: "thanks {sponsor}",
      supportProjectShort: "support",
      supportProjectLong: "support the project",
    },
  },
  notifications: {
    modalTitle: "Notifications",
    dismissAll: "Dismiss all",
  },
  main: {
    energyflow: {
      title: "Current Energyflow",
      houseConsumption: "Consumption",
      pvProduction: "Production",
      battery: "Battery",
      batteryCharge: "Battery charge",
      batteryDischarge: "Battery discharge",
      gridImport: "Grid import",
      selfConsumption: "Self consumption",
      pvExport: "Grid feed-in",
    },
    mode: {
      title: "Mode",
      stop: "Stop",
      now: "Now",
      minpvShort: "Min",
      minpvLong: "Min + PV",
      pvShort: "PV",
      pvLong: "PV only",
    },
    loadpoint: {
      fallbackName: "Loadpoint",
      remoteDisabledSoft: "{source}: adaptive PV charging disabled",
      remoteDisabledHard: "{source}: disabled",
    },
    vehicle: {
      fallbackName: "Vehicle",
    },
    vehicleSoc: {
      disconnected: "disconnected",
      charging: "charging",
      ready: "ready",
      connected: "connected",
    },
    vehicleSubline: {
      mincharge: "minimum charging to {soc}%",
    },
    loadpointDetails: {
      power: "Power",
      range: "Range",
      charged: "Charged",
      duration: "Duration",
      remaining: "Remaining",
    },
  },
};
