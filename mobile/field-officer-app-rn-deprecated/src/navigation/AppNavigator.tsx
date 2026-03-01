import React from 'react'
import { NavigationContainer } from '@react-navigation/native'
import { createBottomTabNavigator } from '@react-navigation/bottom-tabs'
import { createStackNavigator } from '@react-navigation/stack'
import { Ionicons } from '@expo/vector-icons'
import { useAuthStore } from '../store/authStore'

import LoginScreen from '../screens/auth/LoginScreen'
import JobListScreen from '../screens/jobs/JobListScreen'
import MeterCaptureScreen from '../screens/meter/MeterCaptureScreen'
import SOSScreen from '../screens/sos/SOSScreen'

const Tab = createBottomTabNavigator()
const Stack = createStackNavigator()

function JobsStack() {
  return (
    <Stack.Navigator screenOptions={{ headerStyle: { backgroundColor: '#2e7d32' }, headerTintColor: '#fff', headerTitleStyle: { fontWeight: '800' } }}>
      <Stack.Screen name="JobList" component={JobListScreen} options={{ title: 'My Jobs' }} />
      <Stack.Screen name="MeterCapture" component={MeterCaptureScreen} options={{ title: 'Capture Meter' }} />
    </Stack.Navigator>
  )
}

function MainTabs() {
  return (
    <Tab.Navigator
      screenOptions={({ route }) => ({
        tabBarIcon: ({ focused, color, size }) => {
          const icons: Record<string, string> = {
            Jobs: focused ? 'briefcase' : 'briefcase-outline',
            Capture: focused ? 'camera' : 'camera-outline',
            SOS: 'alert-circle',
          }
          return <Ionicons name={icons[route.name] as keyof typeof Ionicons.glyphMap} size={size} color={color} />
        },
        tabBarActiveTintColor: '#2e7d32',
        tabBarInactiveTintColor: '#9ca3af',
        tabBarStyle: { borderTopWidth: 1, borderTopColor: '#f3f4f6', paddingBottom: 4 },
        headerShown: false,
      })}
    >
      <Tab.Screen name="Jobs" component={JobsStack} />
      <Tab.Screen name="Capture" component={MeterCaptureScreen} options={{ title: 'Meter Capture' }} />
      <Tab.Screen
        name="SOS"
        component={SOSScreen}
        options={{
          tabBarLabel: 'SOS',
          tabBarActiveTintColor: '#dc2626',
          tabBarInactiveTintColor: '#dc2626',
        }}
      />
    </Tab.Navigator>
  )
}

export default function AppNavigator() {
  const { isAuthenticated } = useAuthStore()

  return (
    <NavigationContainer>
      <Stack.Navigator screenOptions={{ headerShown: false }}>
        {isAuthenticated ? (
          <Stack.Screen name="Main" component={MainTabs} />
        ) : (
          <Stack.Screen name="Login" component={LoginScreen} />
        )}
      </Stack.Navigator>
    </NavigationContainer>
  )
}
