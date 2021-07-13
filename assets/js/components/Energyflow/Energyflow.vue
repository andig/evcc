<template>
	<div>
		<h4 class="d-none d-md-block my-4">
			{{ $t("main.energyflow.title") }}
		</h4>
		<div
			class="row align-items-start align-items-md-center mt-4 energyflow"
			@click="toggleDetails"
		>
			<Visualization
				class="
					col-12
					offset-md-1
					col-md-6
					offset-lg-1
					col-lg-8
					offset-xl-1
					col-xl-6
					offset-xxl-1
					col-xl-8
					order-md-2
				"
				:showDetails="showDetails"
				:gridImport="gridImport"
				:selfConsumption="selfConsumption"
				:pvExport="pvExport"
				:batteryCharge="batteryCharge"
				:batteryDischarge="batteryDischarge"
				:pvProduction="pvPower"
				:houseConsumption="houseConsumption"
				:batteryIcon="batteryIcon"
			/>
			<div
				class="col-12 col-sm-6 col-md-5 col-lg-3 col-xl-3 order-md-1 mt-2 mt-md-0 small"
				:class="`${showDetails ? 'd-block' : `d-none d-md-block`}`"
			>
				<div class="d-flex justify-content-between" data-test-pv-production>
					<span class="details-icon"><fa-icon icon="sun"></fa-icon></span>
					<span class="text-nowrap flex-grow-1">{{
						$t("main.energyflow.pvProduction")
					}}</span>
					<span class="text-end text-nowrap ps-1">{{ kw(pvPower) }}</span>
				</div>
				<div class="d-flex justify-content-between" data-test-house-consumption>
					<span class="details-icon"><fa-icon icon="home"></fa-icon></span>
					<span class="text-nowrap flex-grow-1">{{
						$t("main.energyflow.houseConsumption")
					}}</span>
					<span class="text-end text-nowrap ps-1">{{ kw(houseConsumption) }}</span>
				</div>
				<div
					v-if="batteryConfigured"
					class="d-flex justify-content-between"
					data-test-battery
				>
					<span class="details-icon">
						<fa-icon :icon="batteryIcon"></fa-icon>
						<fa-icon icon="caret-left" v-if="batteryCharge"></fa-icon>
						<fa-icon icon="caret-right" v-if="batteryDischarge"></fa-icon>
					</span>
					<span class="text-nowrap flex-grow-1 text-truncate">
						<span v-if="batteryCharge">{{ $t("main.energyflow.batteryCharge") }}</span>
						<span v-else-if="batteryDischarge">{{
							$t("main.energyflow.batteryDischarge")
						}}</span>
						<span v-else>{{ $t("main.energyflow.battery") }}</span>
					</span>
					<span class="text-end text-nowrap ps-1">
						({{ batterySoC }}%)
						{{ kw(Math.abs(batteryPower)) }}
					</span>
				</div>
			</div>
			<div
				class="
					col-12 col-sm-6
					offset-md-6
					col-md-6
					offset-lg-4
					col-lg-8
					d-block d-md-flex
					order-md-3
					justify-content-between
					mt-2
					small
				"
				v-if="showDetails"
			>
				<div class="text-nowrap d-flex d-md-block" data-test-grid-import>
					<span class="color-grid details-icon"><fa-icon icon="square"></fa-icon></span>
					<span class="text-nowrap flex-grow-1">{{
						$t("main.energyflow.gridImport")
					}}</span>
					<span class="text-end text-nowrap d-md-none">{{ kw(gridImport) }}</span>
				</div>
				<div class="text-nowrap d-flex d-md-block" data-test-self-consumption>
					<span class="color-self details-icon"><fa-icon icon="square"></fa-icon></span>
					<span class="text-nowrap flex-grow-1">{{
						$t("main.energyflow.selfConsumption")
					}}</span>
					<span class="text-end text-nowrap d-md-none">{{ kw(selfConsumption) }}</span>
				</div>
				<div class="text-nowrap d-flex d-md-block" data-test-pv-export>
					<span class="color-export details-icon"><fa-icon icon="square"></fa-icon></span>
					<span class="text-nowrap flex-grow-1">{{
						$t("main.energyflow.pvExport")
					}}</span>
					<span class="text-end text-nowrap d-md-none">{{ kw(pvExport) }}</span>
				</div>
			</div>
		</div>
	</div>
</template>

<script>
import "../../icons";
import formatter from "../../mixins/formatter";
import Visualization from "./Visualization.vue";

export default {
	name: "Energyflow",
	components: { Visualization },
	props: {
		gridConfigured: Boolean,
		gridPower: { type: Number, default: 0 },
		pvConfigured: Boolean,
		pvPower: { type: Number, default: 0 },
		batteryConfigured: Boolean,
		batteryPower: { type: Number, default: 0 },
		batterySoC: { type: Number, default: 0 },
	},
	data: function () {
		return { showDetails: false };
	},
	mixins: [formatter],
	computed: {
		gridImport: function () {
			return Math.max(0, this.gridPower);
		},
		pvConsumption: function () {
			return Math.min(this.pvPower, this.pvPower + this.gridPower - this.batteryCharge);
		},
		batteryDischarge: function () {
			return Math.max(0, this.batteryPower);
		},
		batteryCharge: function () {
			return Math.min(0, this.batteryPower) * -1;
		},
		houseConsumption: function () {
			return this.gridImport + this.pvConsumption + this.batteryDischarge;
		},
		selfConsumption: function () {
			return this.batteryDischarge + this.pvConsumption + this.batteryCharge;
		},
		pvExport: function () {
			return Math.min(0, this.gridPower) * -1;
		},
		batteryIcon: function () {
			if (this.batterySoC > 80) return "battery-full";
			if (this.batterySoC > 60) return "battery-three-quarters";
			if (this.batterySoC > 40) return "battery-half";
			if (this.batterySoC > 20) return "battery-quarter";
			return "battery-empty";
		},
	},
	methods: {
		kw: function (watt) {
			return Math.max(0, watt / 1000).toFixed(1) + " kW";
		},
		toggleDetails() {
			this.showDetails = !this.showDetails;
		},
	},
};
</script>
<style scoped>
.energyflow {
	cursor: pointer;
}
.color-grid {
	color: var(--evcc-grid);
}
.color-self {
	color: var(--evcc-self);
}
.color-export {
	color: var(--evcc-export);
}
.details-icon {
	text-align: center;
	width: 30px;
	margin-right: 0.25rem;
	white-space: nowrap;
	flex-shrink: 0;
}
</style>
