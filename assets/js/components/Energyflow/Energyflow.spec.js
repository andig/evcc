/* globals describe, it, expect */
import { shallowMount } from "@vue/test-utils";
import Energyflow from "./";

describe("Energyflow.vue", () => {
  const defaultProps = {
    gridConfigured: true,
    gridPower: 0,
    pvConfigured: true,
    pvPower: 0,
    batteryConfigured: false,
    batteryPower: 0,
    batterySoC: 0,
  };

  it("using pv and grid power", async () => {
    const wrapper = shallowMount(Energyflow, {
      mocks: { $t: (x) => x },
      propsData: { ...defaultProps, gridPower: 1000, pvPower: 4000 },
    });

    await wrapper.trigger("click");

    expect(wrapper.find("[data-test-grid-import]").text()).toMatch("1.0 kW");
    expect(wrapper.find("[data-test-self-consumption]").text()).toMatch("4.0 kW");
    expect(wrapper.find("[data-test-pv-export]").text()).toMatch("0.0 kW");

    expect(wrapper.find("[data-test-house-consumption]").text()).toMatch("5.0 kW");
    expect(wrapper.find("[data-test-pv-production]").text()).toMatch("4.0 kW");
    expect(wrapper.find("[data-test-battery]").exists()).toBe(false);
  });

  it("exporting all pv power, no usage", async () => {
    const wrapper = shallowMount(Energyflow, {
      mocks: { $t: (x) => x },
      propsData: { ...defaultProps, gridPower: -4000, pvPower: 4000 },
    });

    await wrapper.trigger("click");

    expect(wrapper.find("[data-test-grid-import]").text()).toMatch("0.0 kW");
    expect(wrapper.find("[data-test-self-consumption]").text()).toMatch("0.0 kW");
    expect(wrapper.find("[data-test-pv-export]").text()).toMatch("4.0 kW");

    expect(wrapper.find("[data-test-house-consumption]").text()).toMatch("0.0 kW");
    expect(wrapper.find("[data-test-pv-production]").text()).toMatch("4.0 kW");
    expect(wrapper.find("[data-test-battery]").exists()).toBe(false);
  });

  it("more grid export than pv, grid value wins (invalid state)", async () => {
    const wrapper = shallowMount(Energyflow, {
      mocks: { $t: (x) => x },
      propsData: { ...defaultProps, gridPower: -4000, pvPower: 3000 },
    });

    await wrapper.trigger("click");

    expect(wrapper.find("[data-test-grid-import]").text()).toMatch("0.0 kW");
    expect(wrapper.find("[data-test-self-consumption]").text()).toMatch("0.0 kW");
    expect(wrapper.find("[data-test-pv-export]").text()).toMatch("4.0 kW");

    expect(wrapper.find("[data-test-house-consumption]").text()).toMatch("0.0 kW");
    expect(wrapper.find("[data-test-pv-production]").text()).toMatch("3.0 kW");
    expect(wrapper.find("[data-test-battery]").exists()).toBe(false);
  });

  it("only grid usage, no pv, idleBattery", async () => {
    const wrapper = shallowMount(Energyflow, {
      mocks: { $t: (x) => x },
      propsData: {
        ...defaultProps,
        gridPower: 360,
        pvPower: 0,
        batteryConfigured: true,
        batteryPower: 0,
      },
    });

    await wrapper.trigger("click");

    expect(wrapper.find("[data-test-grid-import]").text()).toMatch("0.4 kW");
    expect(wrapper.find("[data-test-self-consumption]").text()).toMatch("0.0 kW");
    expect(wrapper.find("[data-test-pv-export]").text()).toMatch("0.0 kW");

    expect(wrapper.find("[data-test-house-consumption]").text()).toMatch("0.4 kW");
    expect(wrapper.find("[data-test-pv-production]").text()).toMatch("0.0 kW");
    expect(wrapper.find("[data-test-battery]").text()).toMatch("main.energyflow.battery");
  });

  it("grid and battery usage, no pv", async () => {
    const wrapper = shallowMount(Energyflow, {
      mocks: { $t: (x) => x },
      propsData: {
        ...defaultProps,
        gridPower: 300,
        batteryConfigured: true,
        batteryPower: 200,
        batterySoC: 77,
        pvPower: 0,
      },
    });

    await wrapper.trigger("click");

    expect(wrapper.find("[data-test-grid-import]").text()).toMatch("0.3 kW");
    expect(wrapper.find("[data-test-self-consumption]").text()).toMatch("0.2 kW");
    expect(wrapper.find("[data-test-pv-export]").text()).toMatch("0.0 kW");

    expect(wrapper.find("[data-test-house-consumption]").text()).toMatch("0.5 kW");
    expect(wrapper.find("[data-test-pv-production]").text()).toMatch("0.0 kW");
    expect(wrapper.find("[data-test-battery]").text()).toMatch("0.2 kW");
    expect(wrapper.find("[data-test-battery]").text()).toMatch("77%");
    expect(wrapper.find("[data-test-battery]").text()).toMatch("main.energyflow.batteryDischarge");
  });

  it("battery charge, pv export", async () => {
    const wrapper = shallowMount(Energyflow, {
      mocks: { $t: (x) => x },
      propsData: {
        ...defaultProps,
        gridPower: -2500,
        batteryConfigured: true,
        batteryPower: -1700,
        pvPower: 9000,
      },
    });

    await wrapper.trigger("click");

    expect(wrapper.find("[data-test-grid-import]").text()).toMatch("0.0 kW");
    expect(wrapper.find("[data-test-self-consumption]").text()).toMatch("6.5 kW");
    expect(wrapper.find("[data-test-pv-export]").text()).toMatch("2.5 kW");

    expect(wrapper.find("[data-test-house-consumption]").text()).toMatch("4.8 kW");
    expect(wrapper.find("[data-test-pv-production]").text()).toMatch("9.0 kW");
    expect(wrapper.find("[data-test-battery]").text()).toMatch("1.7 kW");
    expect(wrapper.find("[data-test-battery]").text()).toMatch("main.energyflow.batteryCharge");
  });

  it("thresholds", async () => {
    const wrapper = shallowMount(Energyflow, {
      mocks: { $t: (x) => x },
      propsData: {
        ...defaultProps,
        gridPower: 5555,
        batteryConfigured: true,
        batteryPower: 1234,
        pvPower: 378,
      },
    });

    await wrapper.trigger("click");

    expect(wrapper.find("[data-test-grid-import]").text()).toMatch("5.6 kW");
    expect(wrapper.find("[data-test-self-consumption]").text()).toMatch("1.6 kW");
    expect(wrapper.find("[data-test-pv-export]").text()).toMatch("0.0 kW");

    expect(wrapper.find("[data-test-house-consumption]").text()).toMatch("7.2 kW");
    expect(wrapper.find("[data-test-pv-production]").text()).toMatch("0.4 kW");
    expect(wrapper.find("[data-test-battery]").text()).toMatch("1.2 kW");
    expect(wrapper.find("[data-test-battery]").text()).toMatch("main.energyflow.batteryDischarge");
  });
});
