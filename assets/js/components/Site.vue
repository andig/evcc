<template>
	<div class="flex-grow-1 d-flex flex-column">
		<Energyflow v-bind="energyflow" />
		<hr class="w-100 my-4" />
		<div class="flex-grow-1 d-flex justify-content-around flex-column">
			<template v-for="(loadpoint, id) in loadpoints">
				<hr class="w-100 my-4" v-if="id > 0" :key="id + '_hr'" />
				<Loadpoint
					:key="id"
					v-bind="loadpoint"
					:single="loadpoints.length === 1"
					:id="id"
				/>
			</template>
		</div>
	</div>
</template>

<script>
import Energyflow from "./Energyflow";
import Loadpoint from "./Loadpoint";
import formatter from "../mixins/formatter";
import collector from "../mixins/collector";

export default {
	name: "Site",
	props: {
		loadpoints: Array,

		// details
		gridConfigured: Boolean,
		gridPower: Number,
		pvConfigured: Boolean,
		pvPower: Number,
		batteryConfigured: Boolean,
		batteryPower: Number,
		batterySoC: Number,
		gridCurrents: Array,
		prioritySoC: Number,
	},
	components: { Loadpoint, Energyflow },
	mixins: [formatter, collector],
	computed: {
		energyflow: function () {
			return this.collectProps(Energyflow);
		},
	},
};
</script>
