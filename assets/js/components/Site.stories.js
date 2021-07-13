import Site from "./Site.vue";
import i18n from "../i18n";

export default {
  title: "Main/Site",
  component: Site,
  argTypes: {},
};

const Template = (args, { argTypes }) => ({
  i18n,
  props: Object.keys(argTypes),
  components: { Site },
  template: '<Site v-bind="$props"></Site>',
});

export const Base = Template.bind({});
Base.args = {
  gridConfigured: true,
  pvConfigured: true,
  gridPower: 1340,
  pvPower: 6420,
  batteryConfigured: true,
  batteryPower: 800,
  batterySoC: 66,
  loadpoints: [],
};

export const Single = Template.bind({});
Single.args = {
  gridConfigured: true,
  pvConfigured: true,
  gridPower: -2300,
  pvPower: 7000,
  batteryConfigured: true,
  batteryPower: -2000,
  batterySoC: 24,
  loadpoints: [
    {
      title: "Ladepunkt 1",
      mode: "now",
      socTitle: "Mein Auto",
      enabled: true,
      connected: true,
      hasVehicle: true,
      charging: true,
      socCharge: 66,
      targetSoC: 90,
      range: 344,
      chargeEstimate: 999,
      chargePower: 11232,
      chargeDuration: 123982,
      chargedEnergy: 23213,
    },
  ],
};

export const Multi = Template.bind({});
Multi.args = {
  gridConfigured: true,
  pvConfigured: true,
  gridPower: -1000,
  pvPower: 5500,
  batteryConfigured: false,
  batteryPower: 70,
  batterySoC: 60,
  loadpoints: [
    {
      title: "Ladepunkt 1",
      mode: "now",
      socTitle: "Mein Auto",
      enabled: true,
      connected: true,
      hasVehicle: true,
      charging: true,
      socCharge: 66,
      targetSoC: 90,
      range: 344,
      chargeEstimate: 999,
      chargePower: 11232,
      chargeDuration: 123982,
      chargedEnergy: 23213,
    },
    {
      title: "Ladepunkt 2",
      mode: "pv",
    },
  ],
};
