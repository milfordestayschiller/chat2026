<template>
    <div class="modal" :class="{ 'is-active': visible }">
      <div class="modal-background"></div>
      <div class="modal-content">
        <div class="box" style="width: 400px; margin: auto;">
          
          <!-- LOGIN FORM -->
          <div v-if="view === 'login'">
            <h2 class="title is-4 has-text-centered">Iniciar Sesión</h2>
  
            <div class="field">
              <label class="label">Username</label>
              <input class="input" type="text" v-model="username" placeholder="Ingresa tu username" />
            </div>
  
            <div class="field">
              <label class="label">Contraseña</label>
              <input class="input" type="password" v-model="password" placeholder="Ingresa tu contraseña" />
            </div>
  
            <div class="buttons mt-4 is-flex is-justify-content-space-between">
              <button class="button is-link" @click="handleLogin">Iniciar Sesión</button>
              <button class="button is-info" @click="view = 'register'">Suscríbete</button>
              <button class="button is-light" @click="view = 'guest'">Invitado</button>
            </div>
          </div>
  
          <!-- REGISTER FORM -->
          <div v-else-if="view === 'register'">
            <h2 class="title is-4 has-text-centered">Registro</h2>
  
            <div class="field">
              <label class="label">Nuevo Nick</label>
              <input class="input" type="text" v-model="registerUsername" placeholder="Nuevo usuario" />
            </div>
  
            <div class="field">
              <label class="label">Contraseña</label>
              <input class="input" type="password" v-model="registerPassword" placeholder="Contraseña" />
            </div>
  
            <div class="buttons mt-4 is-flex is-justify-content-space-between">
              <button class="button is-success" @click="handleRegister">Registrar</button>
              <button class="button is-light" @click="view = 'login'">Cerrar</button>
            </div>
          </div>
  
          <!-- GUEST FORM -->
          <div v-else-if="view === 'guest'">
            <h2 class="title is-4 has-text-centered">Entrar como Invitado</h2>
  
            <div class="field">
              <label class="label">Nick de invitado</label>
              <input class="input" type="text" v-model="guestUsername" placeholder="Ingresa tu nick" />
            </div>
  
            <form @submit.prevent="handleGuest">
              <div class="field">
                <div class="control">
                  <button class="button is-link">Entrar al Chat</button>
                </div>
              </div>
            </form>
          </div>
  
        </div>
      </div>
    </div>
  </template>
  
  <script>
  export default {
    props: {
      visible: Boolean,
    },
    data() {
      return {
        view: 'login',
        username: '',
        password: '',
        registerUsername: '',
        registerPassword: '',
        guestUsername: '',
      };
    },
    methods: {
      signIn(username) {
        this.$emit('signIn', username);
      },
      async handleLogin() {
        if (!this.username || !this.password) {
          alert('Completa los campos para iniciar sesión.');
          return;
        }
  
        try {
          const res = await fetch('/api/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: new URLSearchParams({
              username: this.username,
              password: this.password,
            }),
          });
  
          if (res.ok) {
            this.signIn(this.username);
          } else {
            const msg = await res.text();
            alert('Error de inicio de sesión: ' + msg);
          }
        } catch (err) {
          alert('Error de red al iniciar sesión.');
          console.error(err);
        }
      },
      async handleRegister() {
        if (!this.registerUsername || !this.registerPassword) {
          alert('Completa ambos campos para registrarte.');
          return;
        }
  
        try {
          const res = await fetch('/api/register', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: new URLSearchParams({
              username: this.registerUsername,
              password: this.registerPassword,
            }),
          });
  
          if (res.ok) {
            alert('Usuario registrado correctamente.');
            this.username = this.registerUsername;
            this.password = this.registerPassword;
            this.view = 'login';
          } else {
            const msg = await res.text();
            alert('Error al registrar: ' + msg);
          }
        } catch (err) {
          alert('Error de red al registrar.');
          console.error(err);
        }
      },
      handleGuest() {
        if (this.guestUsername) {
          this.signIn(this.guestUsername);
        } else {
          alert('Ingresa un nick para continuar como invitado.');
        }
      }
    }
  };
  </script>
  
  <style scoped>
  .modal-content {
    display: flex;
    justify-content: center;
    align-items: center;
  }
  </style>
  