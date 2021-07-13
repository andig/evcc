import Energyflow from "./";
import i18n from "../../i18n";

export default {
  title: "Main/Energyflow",
  component: Energyflow,
  argTypes: {
    gridPower: { control: { type: "range", min: -5000, max: 20000, step: 100 } },
    pvPower: { control: { type: "range", min: 0, max: 10000, step: 100 } },
    batteryPower: { control: { type: "range", min: -4000, max: 4000, step: 100 } },
    batterySoC: { control: { type: "range", min: 0, max: 100, step: 1 } },
  },
};

const Template = (args, { argTypes }) => ({
  i18n,
  props: Object.keys(argTypes),
  components: { Energyflow },
  template: '<Energyflow v-bind="$props"></Energyflow>',
});

export const GridAndPV = Template.bind({});
GridAndPV.args = {
  gridConfigured: true,
  pvConfigured: true,
  gridPower: -2300,
  pvPower: 7320,
  batteryConfigured: false,
};

export const BatteryAndGrid = Template.bind({});
BatteryAndGrid.args = {
  gridConfigured: true,
  pvConfigured: true,
  batteryConfigured: true,
  gridPower: 1200,
  pvPower: 0,
  batteryPower: 800,
  batterySoC: 77,
};

export const BatteryCharging = Template.bind({});
BatteryCharging.args = {
  gridConfigured: true,
  pvConfigured: true,
  batteryConfigured: true,
  gridPower: -1300,
  pvPower: 5000,
  batteryPower: -1500,
  batterySoC: 75,
};

export const GridPvAndBattery = Template.bind({});
GridPvAndBattery.args = {
  gridConfigured: true,
  pvConfigured: true,
  batteryConfigured: true,
  gridPower: 700,
  pvPower: 1000,
  batteryPower: 1500,
  batterySoC: 30,
};

export const BatteryThresholds = Template.bind({});
BatteryThresholds.args = {
  gridConfigured: true,
  pvConfigured: true,
  batteryConfigured: true,
  gridPower: -510,
  pvPower: 8740,
  batteryPower: -100,
  batterySoC: 95,
};

export const PvThresholds = Template.bind({});
PvThresholds.args = {
  gridConfigured: true,
  pvConfigured: true,
  batteryConfigured: true,
  batteryPower: 800,
  gridPower: 5555,
  pvPower: 300,
  batterySoC: 76,
};

export const GridOnly = Template.bind({});
GridOnly.args = {
  gridConfigured: true,
  pvConfigured: false,
  batteryConfigured: false,
  gridPower: -6230,
  pvPower: 0,
  batteryPower: 0,
  batterySoC: 0,
};
