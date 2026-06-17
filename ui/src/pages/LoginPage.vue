<template>
  <div class="login-page column items-center justify-center">
    <div class="login-card q-pa-xl" style="width: 400px; max-width: 90vw;">
      <div class="column items-center q-mb-lg">
        <img src="/icon.png" alt="Cryo" style="width: 64px; height: 64px;" />
        <span class="text-h5 text-weight-medium q-mt-sm">Cryo</span>
        <span class="text-caption text-grey-6">Sign in to continue</span>
      </div>

      <q-form v-if="authInfo?.basicEnabled" @submit.prevent="onLogin" class="q-gutter-md">
        <q-input
          v-model="username"
          label="Username"
          outlined
          dense
          :error="!!loginError"
        />
        <q-input
          v-model="password"
          label="Password"
          type="password"
          outlined
          dense
          :error="!!loginError"
          :error-message="loginError"
        />
        <q-btn
          type="submit"
          label="Sign in"
          color="primary"
          class="full-width"
          :loading="loading"
        />
      </q-form>

      <template v-if="authInfo?.basicEnabled && authInfo?.oidcEnabled">
        <q-separator class="q-my-md" />
        <div class="text-center text-caption text-grey-6 q-mb-sm">or</div>
      </template>

      <q-btn
        v-if="authInfo?.oidcEnabled"
        label="Sign in with SSO"
        color="secondary"
        outline
        class="full-width"
        :href="authInfo.oidcLoginUrl"
        no-caps
      />

      <div v-if="!authInfo" class="text-center q-pa-md">
        <q-spinner size="24px" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import authApi, { type AuthInfo } from 'src/api/auth';

const router = useRouter();
const authInfo = ref<AuthInfo | null>(null);
const username = ref('');
const password = ref('');
const loginError = ref('');
const loading = ref(false);

onMounted(async () => {
  try {
    authInfo.value = await authApi.getAuthInfo();
    if (!authInfo.value.basicEnabled && !authInfo.value.oidcEnabled) {
      router.replace({ name: 'home' });
    }
  } catch {
    loginError.value = 'Failed to load auth configuration';
  }
});

async function onLogin() {
  loginError.value = '';
  loading.value = true;
  try {
    await authApi.login(username.value, password.value);
    router.replace({ name: 'home' });
  } catch (e: unknown) {
    const err = e as { response?: { status?: number } };
    if (err.response?.status === 401) {
      loginError.value = 'Invalid username or password';
    } else {
      loginError.value = 'Login failed';
    }
  } finally {
    loading.value = false;
  }
}
</script>

<style lang="scss" scoped>
.login-page {
  min-height: 100vh;
  background: #f5f5f5;
}

.login-card {
  background: white;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);
}
</style>
