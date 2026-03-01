import React, { useState } from 'react'
import {
  View, Text, TextInput, TouchableOpacity, StyleSheet,
  KeyboardAvoidingView, Platform, ActivityIndicator, Alert,
} from 'react-native'
import * as LocalAuthentication from 'expo-local-authentication'
import * as SecureStore from 'expo-secure-store'
import { useAuthStore } from '../../store/authStore'
import { apiClient } from '../../utils/api'

export default function LoginScreen() {
  const { login } = useAuthStore()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)

  const handleLogin = async () => {
    if (!email || !password) {
      Alert.alert('Error', 'Please enter your email and password.')
      return
    }
    setLoading(true)
    try {
      const res = await apiClient.post('/api/v1/auth/login', { email, password })
      const { token, user, refresh_token } = res.data.data
      // Store refresh token for future biometric logins
      if (refresh_token) {
        await SecureStore.setItemAsync('gnwaas_refresh_token', refresh_token)
      }
      await login(token, user)
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { message?: string } } })?.response?.data?.message
      Alert.alert('Login Failed', msg || 'Invalid credentials. Please try again.')
    } finally {
      setLoading(false)
    }
  }

  const handleBiometric = async () => {
    const compatible = await LocalAuthentication.hasHardwareAsync()
    if (!compatible) {
      Alert.alert('Not Available', 'Biometric authentication is not available on this device.')
      return
    }

    // Check if we have a stored refresh token to exchange
    const storedToken = await SecureStore.getItemAsync('gnwaas_refresh_token')
    if (!storedToken) {
      Alert.alert(
        'Sign In First',
        'Please sign in with your password once to enable biometric login.',
      )
      return
    }

    const result = await LocalAuthentication.authenticateAsync({
      promptMessage: 'Authenticate to access GN-WAAS',
      fallbackLabel: 'Use Password',
      disableDeviceFallback: false,
    })

    if (!result.success) {
      if (result.error !== 'user_cancel') {
        Alert.alert('Authentication Failed', 'Biometric verification was not successful.')
      }
      return
    }

    // Biometric verified — exchange stored refresh token for a fresh JWT
    setLoading(true)
    try {
      const res = await apiClient.post('/api/v1/auth/refresh', {
        refresh_token: storedToken,
        grant_type: 'refresh_token',
      })
      const { token, user } = res.data.data
      // Store new refresh token if provided
      if (res.data.data.refresh_token) {
        await SecureStore.setItemAsync('gnwaas_refresh_token', res.data.data.refresh_token)
      }
      await login(token, user)
    } catch (err: unknown) {
      // Refresh token expired — force full password login
      await SecureStore.deleteItemAsync('gnwaas_refresh_token')
      Alert.alert(
        'Session Expired',
        'Your session has expired. Please sign in with your password.',
      )
    } finally {
      setLoading(false)
    }
  }

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === 'ios' ? 'padding' : 'height'}
    >
      <View style={styles.inner}>
        {/* Logo */}
        <View style={styles.logoContainer}>
          <View style={styles.logoBox}>
            <Text style={styles.logoText}>💧</Text>
          </View>
          <Text style={styles.appName}>GN-WAAS</Text>
          <Text style={styles.appSubtitle}>Field Officer App</Text>
        </View>

        {/* Form */}
        <View style={styles.form}>
          <Text style={styles.label}>Email Address</Text>
          <TextInput
            style={styles.input}
            value={email}
            onChangeText={setEmail}
            placeholder="officer@gwl.gov.gh"
            keyboardType="email-address"
            autoCapitalize="none"
            autoComplete="email"
          />

          <Text style={styles.label}>Password</Text>
          <TextInput
            style={styles.input}
            value={password}
            onChangeText={setPassword}
            placeholder="••••••••"
            secureTextEntry
          />

          <TouchableOpacity
            style={[styles.loginBtn, loading && styles.loginBtnDisabled]}
            onPress={handleLogin}
            disabled={loading}
          >
            {loading ? (
              <ActivityIndicator color="#fff" />
            ) : (
              <Text style={styles.loginBtnText}>Sign In</Text>
            )}
          </TouchableOpacity>

          <TouchableOpacity style={styles.biometricBtn} onPress={handleBiometric}>
            <Text style={styles.biometricText}>🔐  Use Biometric / Face ID</Text>
          </TouchableOpacity>
        </View>

        <Text style={styles.footer}>
          GN-WAAS · Ghana National Water Audit System{'\n'}
          Authorised personnel only
        </Text>
      </View>
    </KeyboardAvoidingView>
  )
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: '#1b5e20' },
  inner: { flex: 1, justifyContent: 'center', paddingHorizontal: 28 },
  logoContainer: { alignItems: 'center', marginBottom: 40 },
  logoBox: {
    width: 72, height: 72, backgroundColor: '#fdd835',
    borderRadius: 18, alignItems: 'center', justifyContent: 'center', marginBottom: 12,
  },
  logoText: { fontSize: 36 },
  appName: { fontSize: 28, fontWeight: '900', color: '#fff' },
  appSubtitle: { fontSize: 14, color: '#a5d6a7', marginTop: 4 },
  form: {
    backgroundColor: '#fff', borderRadius: 20, padding: 24,
    shadowColor: '#000', shadowOffset: { width: 0, height: 8 },
    shadowOpacity: 0.2, shadowRadius: 16, elevation: 8,
  },
  label: { fontSize: 13, fontWeight: '600', color: '#374151', marginBottom: 6 },
  input: {
    borderWidth: 1, borderColor: '#e5e7eb', borderRadius: 10,
    paddingHorizontal: 14, paddingVertical: 12, fontSize: 14,
    marginBottom: 16, backgroundColor: '#f9fafb',
  },
  loginBtn: {
    backgroundColor: '#2e7d32', borderRadius: 12,
    paddingVertical: 14, alignItems: 'center', marginTop: 4,
  },
  loginBtnDisabled: { opacity: 0.6 },
  loginBtnText: { color: '#fff', fontSize: 15, fontWeight: '700' },
  biometricBtn: {
    marginTop: 12, paddingVertical: 12, alignItems: 'center',
    borderWidth: 1, borderColor: '#e5e7eb', borderRadius: 12,
  },
  biometricText: { fontSize: 14, color: '#6b7280', fontWeight: '500' },
  footer: { textAlign: 'center', color: '#81c784', fontSize: 11, marginTop: 32, lineHeight: 18 },
})
