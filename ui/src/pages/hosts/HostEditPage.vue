<template>
  <q-page padding>
    <!-- Loading state -->
    <div v-if="loading" class="text-center q-pa-xl">
      <q-spinner-dots color="primary" size="40px" />
      <div class="text-grey q-mt-sm">Loading host...</div>
    </div>

    <!-- Error state -->
    <div v-else-if="loadError" class="text-center q-pa-xl">
      <q-icon name="error_outline" color="negative" size="48px" />
      <div class="text-negative q-mt-sm">{{ loadError }}</div>
      <q-btn class="q-mt-md" color="primary" label="Back" no-caps outline :to="{ name: 'hosts' }" />
    </div>

    <template v-else>
      <div class="row items-center q-mb-lg">
        <q-btn flat round icon="arrow_back" color="grey" :to="{ name: 'host-detail', params: { namespace, name } }" class="q-mr-sm" />
        <div class="text-h5 text-weight-medium">Edit Host</div>
      </div>

      <q-card flat bordered class="cryo-card" style="max-width: 600px;">
        <q-card-section>
          <q-form @submit.prevent="onSubmit" class="q-gutter-md">
            <!-- Name (read-only) -->
            <q-input
              :model-value="name"
              label="Secret Name"
              outlined
              disable
            />

            <!-- Namespace (read-only) -->
            <q-input
              :model-value="namespace"
              label="Namespace"
              outlined
              disable
            />

            <!-- Host type selector -->
            <q-select
              v-model="hostType"
              :options="hostTypeOptions"
              label="Host Type"
              outlined
              emit-value
              map-options
            />

            <!-- Base URL -->
            <q-input
              v-model="form.baseUrl"
              label="Base URL"
              :hint="baseUrlHint"
              :placeholder="baseUrlPlaceholder"
              outlined
              :rules="[val => !!val || 'Base URL is required']"
            />

            <!-- S3 credentials (shown when type is s3) -->
            <template v-if="hostType === 's3'">
              <q-separator class="q-my-sm" />
              <div class="text-subtitle2 text-grey">S3 Credentials</div>

              <q-input
                v-model="form.awsAccessKeyId"
                label="AWS Access Key ID"
                hint="Leave empty to keep current"
                outlined
              />
              <q-input
                v-model="form.awsSecretAccessKey"
                label="AWS Secret Access Key"
                hint="Leave empty to keep current"
                outlined
                :type="showAwsSecret ? 'text' : 'password'"
              >
                <template v-slot:append>
                  <q-icon
                    :name="showAwsSecret ? 'visibility_off' : 'visibility'"
                    class="cursor-pointer"
                    @click="showAwsSecret = !showAwsSecret"
                  />
                </template>
              </q-input>
              <q-input
                v-model="form.awsDefaultRegion"
                label="AWS Region"
                hint="Leave empty to keep current"
                placeholder="eu-central-1"
                outlined
              />
            </template>

            <!-- Submit -->
            <div class="row justify-end q-mt-lg">
              <q-btn
                label="Cancel"
                no-caps
                flat
                color="grey"
                :to="{ name: 'host-detail', params: { namespace, name } }"
                class="q-mr-sm"
              />
              <q-btn
                type="submit"
                label="Save Changes"
                color="primary"
                no-caps
                unelevated
                :loading="submitting"
              />
            </div>
          </q-form>
        </q-card-section>
      </q-card>

      <!-- Error banner -->
      <q-banner v-if="submitError" class="bg-negative text-white q-mt-md" rounded>
        <template v-slot:avatar>
          <q-icon name="error" />
        </template>
        {{ submitError }}
      </q-banner>
    </template>
  </q-page>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import hostsApi, { type HostType } from 'src/api/hosts';

const route = useRoute();
const router = useRouter();
const namespace = route.params.namespace as string;
const name = route.params.name as string;

const loading = ref(true);
const loadError = ref('');

const hostType = ref<HostType>('s3');
const hostTypeOptions = [
  { label: 'S3', value: 's3' },
  { label: 'Local', value: 'local' },
  { label: 'SFTP', value: 'sftp' },
  { label: 'REST Server', value: 'rest' },
];

const form = ref({
  baseUrl: '',
  awsAccessKeyId: '',
  awsSecretAccessKey: '',
  awsDefaultRegion: '',
});

const showAwsSecret = ref(false);
const submitting = ref(false);
const submitError = ref('');

const baseUrlHint = computed(() => {
  switch (hostType.value) {
    case 's3': return 'e.g. s3:https://s3.amazonaws.com';
    case 'sftp': return 'e.g. sftp:user@host:';
    case 'rest': return 'e.g. rest:https://host:8000';
    case 'local': return 'e.g. /srv/restic';
    default: return '';
  }
});

const baseUrlPlaceholder = computed(() => {
  switch (hostType.value) {
    case 's3': return 's3:https://s3.amazonaws.com';
    case 'sftp': return 'sftp:user@host:';
    case 'rest': return 'rest:https://host:8000';
    case 'local': return '/srv/restic';
    default: return '';
  }
});

async function onSubmit() {
  submitting.value = true;
  submitError.value = '';
  try {
    await hostsApi.updateHost(namespace, name, {
      baseUrl: form.value.baseUrl,
      awsAccessKeyId: form.value.awsAccessKeyId,
      awsSecretAccessKey: form.value.awsSecretAccessKey,
      awsDefaultRegion: form.value.awsDefaultRegion,
    });
    router.push({ name: 'host-detail', params: { namespace, name } });
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : 'Unknown error';
    submitError.value = `Failed to update host: ${msg}`;
    console.error('Failed to update host:', e);
  } finally {
    submitting.value = false;
  }
}

onMounted(async () => {
  try {
    const host = await hostsApi.getHost(namespace, name);
    hostType.value = host.type;
    form.value.baseUrl = host.baseUrl;
  } catch (e) {
    loadError.value = 'Failed to load host.';
    console.error('Failed to load host:', e);
  } finally {
    loading.value = false;
  }
});
</script>
